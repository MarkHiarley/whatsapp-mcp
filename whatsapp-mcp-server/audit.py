"""Privacy-preserving audit trail for MCP tool calls.

Only metadata is recorded: tool, outcome, duration, tenant, approval id and a
hash of the recipient. Message text, full JIDs, file paths and tokens never go
to the audit log.
"""

from __future__ import annotations

import hashlib
import inspect
import json
import os
import threading
import time
from functools import wraps
from pathlib import Path
from typing import Any, Callable

from approvals import state_dir

_LOCK = threading.Lock()


def audit_path() -> Path:
    configured = os.getenv("WHATSAPP_MCP_AUDIT_LOG")
    if configured:
        return Path(configured).expanduser().resolve()
    return state_dir() / "audit.jsonl"


def _append(record: dict[str, Any]) -> None:
    path = audit_path()
    try:
        path.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
        line = json.dumps(record, ensure_ascii=False, separators=(",", ":"))
        with _LOCK:
            with path.open("a", encoding="utf-8") as handle:
                handle.write(line + "\n")
            try:
                path.chmod(0o600)
            except OSError:
                pass
    except OSError:
        # Auditing must never turn a successful tool call into a failure.
        return


def _recipient_hash(value: Any) -> str | None:
    if not isinstance(value, str) or not value:
        return None
    return hashlib.sha256(value.encode("utf-8")).hexdigest()[:16]


def _fields(fn: Callable[..., Any], args: tuple[Any, ...], kwargs: dict[str, Any]) -> dict[str, Any]:
    try:
        bound = inspect.signature(fn).bind_partial(*args, **kwargs)
    except (TypeError, ValueError):
        return {}
    arguments = bound.arguments
    fields: dict[str, Any] = {}
    if "recipient" in arguments:
        fields["recipient_hash"] = _recipient_hash(arguments["recipient"])
    if "approval_id" in arguments:
        fields["approval_id"] = arguments["approval_id"]
    if "media_path" in arguments and isinstance(arguments["media_path"], str):
        fields["file_name"] = Path(arguments["media_path"]).name
    return fields


def audited(fn: Callable[..., Any]) -> Callable[..., Any]:
    """Wrap one MCP tool with a metadata-only audit record."""

    @wraps(fn)
    def wrapper(*args: Any, **kwargs: Any) -> Any:
        started = time.monotonic()
        record: dict[str, Any] = {
            "ts": time.time(),
            "tool": fn.__name__,
            "tenant": os.getenv("WHATSAPP_TENANT_ID", "default"),
            **_fields(fn, args, kwargs),
        }
        try:
            result = fn(*args, **kwargs)
            record["outcome"] = "ok"
            return result
        except Exception as exc:  # noqa: BLE001 - logged and re-raised
            record["outcome"] = "error"
            record["error"] = type(exc).__name__
            raise
        finally:
            record["duration_ms"] = round((time.monotonic() - started) * 1000, 1)
            _append(record)

    return wrapper
