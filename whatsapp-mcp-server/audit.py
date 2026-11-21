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
import whatsapp

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


def _scopes() -> list[str] | None:
    """Return the bearer scopes, or None for trusted STDIO/env mode."""
    try:
        from mcp.server.auth.middleware.auth_context import get_access_token
        token = get_access_token()
    except Exception:
        return None
    return None if token is None else list(token.scopes)


def _allowed(scope: str) -> bool:
    scopes = _scopes()
    return scopes is None or scope in scopes


def audited(fn: Callable[..., Any] | None = None, *, required_scope: str | None = None):
    """Wrap one MCP tool with audit metadata and optional scope enforcement."""

    def decorate(function: Callable[..., Any]) -> Callable[..., Any]:
        @wraps(function)
        def wrapper(*args: Any, **kwargs: Any) -> Any:
            whatsapp.bind_from_access_token()
            started = time.monotonic()
            record: dict[str, Any] = {
                "ts": time.time(),
                "tool": function.__name__,
                "tenant": whatsapp.current_tenant(),
                **_fields(function, args, kwargs),
            }
            if required_scope is not None and not _allowed(required_scope):
                record["outcome"] = "denied"
                record["error"] = "PermissionError"
                record["duration_ms"] = round((time.monotonic() - started) * 1000, 1)
                _append(record)
                raise PermissionError(f"scope {required_scope!r} is required")
            try:
                result = function(*args, **kwargs)
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

    if fn is None:
        return decorate
    return decorate(fn)
