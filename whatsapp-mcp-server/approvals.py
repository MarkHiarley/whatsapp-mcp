"""Two-step approval store for outgoing WhatsApp actions.

The MCP server never sends a draft directly. It creates a short-lived approval
row here and only ``confirm_send`` can consume it. The approval is single-use,
bound to the tenant/recipient, and re-checks file hashes before sending.
"""

from __future__ import annotations

import hashlib
import os
import secrets
import sqlite3
import time
from pathlib import Path
from typing import Any, Optional

APPROVAL_PREFIX = "ap_"
DEFAULT_TTL_SECONDS = 120
MAX_TTL_SECONDS = 3600
DEFAULT_MAX_FILE_BYTES = 64 * 1024 * 1024


def _env_int(name: str, default: int, minimum: int, maximum: int) -> int:
    raw = os.getenv(name, "")
    try:
        value = int(raw) if raw else default
    except ValueError:
        value = default
    return max(minimum, min(maximum, value))


def approval_ttl_seconds() -> int:
    return _env_int("WHATSAPP_MCP_APPROVAL_TTL_SECONDS", DEFAULT_TTL_SECONDS, 30, MAX_TTL_SECONDS)


def max_file_bytes() -> int:
    return _env_int("WHATSAPP_MCP_MAX_FILE_MB", DEFAULT_MAX_FILE_BYTES // (1024 * 1024), 1, 100) * 1024 * 1024


def state_dir() -> Path:
    """Persistent MCP state, preferably inside the bridge store volume."""
    configured = os.getenv("WHATSAPP_MCP_STATE_DIR")
    if configured:
        path = Path(configured).expanduser()
    else:
        store = os.getenv("WHATSAPP_STORE_DIR")
        base = Path(store).expanduser() if store else Path(__file__).resolve().parent.parent / "whatsapp-bridge" / "store"
        path = base / "mcp"
    path.mkdir(parents=True, exist_ok=True, mode=0o700)
    return path.resolve()


def database_path() -> Path:
    configured = os.getenv("WHATSAPP_MCP_APPROVALS_DB")
    if configured:
        return Path(configured).expanduser().resolve()
    return state_dir() / "approvals.db"


def outbox_dir() -> Path:
    configured = os.getenv("WHATSAPP_MCP_OUTBOX_DIR")
    path = Path(configured).expanduser() if configured else state_dir() / "outbox"
    path.mkdir(parents=True, exist_ok=True, mode=0o700)
    return path.resolve()


def _connect() -> sqlite3.Connection:
    path = database_path()
    path.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
    conn = sqlite3.connect(str(path), timeout=10)
    conn.row_factory = sqlite3.Row
    conn.execute(
        """
        CREATE TABLE IF NOT EXISTS approvals (
            id TEXT PRIMARY KEY,
            tenant_id TEXT NOT NULL,
            kind TEXT NOT NULL,
            recipient TEXT NOT NULL,
            text TEXT,
            file_path TEXT,
            file_sha256 TEXT,
            created_at REAL NOT NULL,
            expires_at REAL NOT NULL,
            used_at REAL
        )
        """
    )
    conn.execute("CREATE INDEX IF NOT EXISTS approvals_expiry ON approvals (expires_at)")
    return conn


def _cleanup(conn: sqlite3.Connection, now: float) -> None:
    conn.execute("DELETE FROM approvals WHERE expires_at < ? AND used_at IS NOT NULL", (now - 86400,))


def hash_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def resolve_outbox_file(raw_path: str) -> Path:
    """Resolve a send path inside the outbox; reject escapes, symlinks and huge files."""
    raw = (raw_path or "").strip()
    if not raw:
        raise ValueError("media_path must be provided")

    root = outbox_dir()
    candidate = Path(raw).expanduser()
    if not candidate.is_absolute():
        candidate = root / candidate
    if candidate.is_symlink():
        raise ValueError("symlinks are not allowed in the outbox")

    resolved = candidate.resolve()
    try:
        resolved.relative_to(root)
    except ValueError as exc:
        raise ValueError(f"media_path must be inside {root}") from exc

    if not resolved.is_file():
        raise ValueError(f"media file not found: {resolved.name}")
    if resolved.stat().st_size > max_file_bytes():
        raise ValueError(f"media file exceeds {max_file_bytes() // (1024 * 1024)} MiB")
    return resolved


def create(
    *,
    tenant_id: str,
    kind: str,
    recipient: str,
    text: Optional[str] = None,
    file_path: Optional[Path] = None,
    file_sha256: Optional[str] = None,
) -> dict[str, Any]:
    now = time.time()
    approval_id = APPROVAL_PREFIX + secrets.token_urlsafe(24)
    expires_at = now + approval_ttl_seconds()
    with _connect() as conn:
        _cleanup(conn, now)
        conn.execute(
            """
            INSERT INTO approvals
                (id, tenant_id, kind, recipient, text, file_path, file_sha256, created_at, expires_at)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
            """,
            (
                approval_id,
                tenant_id,
                kind,
                recipient,
                text,
                str(file_path) if file_path else None,
                file_sha256,
                now,
                expires_at,
            ),
        )
    return {
        "approval_id": approval_id,
        "tenant_id": tenant_id,
        "kind": kind,
        "recipient": recipient,
        "text": text,
        "file_path": str(file_path) if file_path else None,
        "file_sha256": file_sha256,
        "created_at": now,
        "expires_at": expires_at,
    }


def consume(*, tenant_id: str, approval_id: str) -> Optional[dict[str, Any]]:
    """Atomically consume one approval; returns None when missing, expired or used."""
    now = time.time()
    with _connect() as conn:
        cursor = conn.execute(
            """
            UPDATE approvals SET used_at = ?
            WHERE id = ? AND tenant_id = ? AND used_at IS NULL AND expires_at > ?
            """,
            (now, approval_id, tenant_id, now),
        )
        if cursor.rowcount != 1:
            return None
        row = conn.execute("SELECT * FROM approvals WHERE id = ?", (approval_id,)).fetchone()
        return dict(row) if row else None


def file_still_matches(approval: dict[str, Any]) -> bool:
    raw_path = approval.get("file_path")
    expected = approval.get("file_sha256")
    if not raw_path or not expected:
        return False
    path = Path(str(raw_path))
    if path.is_symlink() or not path.is_file():
        return False
    try:
        path.relative_to(outbox_dir())
    except ValueError:
        return False
    return hash_file(path) == expected


def iso_from_epoch(value: float) -> str:
    from datetime import datetime, timezone

    return datetime.fromtimestamp(value, tz=timezone.utc).isoformat()
