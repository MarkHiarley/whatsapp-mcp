"""Remote MCP configuration: Streamable HTTP and per-token tenant scopes.

The server listens on the MCP endpoint (default ``/mcp``). Every request must
carry a bearer token from ``WHATSAPP_MCP_AUTH_TOKENS_FILE`` (mode 600) or
``WHATSAPP_MCP_AUTH_TOKENS``. The token maps to one tenant and a scope list;
the tenant's bridge key never reaches the model.
"""

from __future__ import annotations

import json
import os
import stat
import time
from pathlib import Path
from typing import Any

import httpx

from mcp.server.auth.middleware.auth_context import get_access_token
from mcp.server.auth.provider import AccessToken, TokenVerifier
from mcp.server.auth.settings import AuthSettings
from mcp.server.fastmcp import FastMCP
from mcp.server.transport_security import TransportSecuritySettings

import whatsapp

ALLOWED_SCOPES = {"read", "write"}
WRITE_TOOL_NAMES = {
    "send_message", "send_file", "send_audio_message",
    "draft_message", "draft_file", "draft_audio_message", "confirm_send",
    "ack_events",
}


def _env_flag(name: str, default: bool = False) -> bool:
    raw = os.getenv(name)
    if raw is None:
        return default
    return raw.strip().lower() in {"1", "true", "yes", "on"}


def transport() -> str:
    value = (os.getenv("WHATSAPP_MCP_TRANSPORT") or "stdio").strip().lower()
    if value in {"http", "streamable-http", "streamable_http"}:
        return "streamable-http"
    return "stdio"


def http_host() -> str:
    return os.getenv("WHATSAPP_MCP_HTTP_HOST") or "127.0.0.1"


def http_port() -> int:
    try:
        port = int(os.getenv("WHATSAPP_MCP_HTTP_PORT") or "8765")
    except ValueError:
        port = 8765
    return max(1, min(65535, port))


def http_path() -> str:
    value = (os.getenv("WHATSAPP_MCP_HTTP_PATH") or "/mcp").strip()
    return value if value.startswith("/") else "/" + value


def public_url() -> str:
    configured = os.getenv("WHATSAPP_MCP_PUBLIC_URL")
    if configured:
        return configured.rstrip("/")
    host = http_host()
    display_host = "127.0.0.1" if host in {"0.0.0.0", "::"} else host
    return f"http://{display_host}:{http_port()}"


def load_tokens() -> dict[str, dict[str, Any]]:
    raw = os.getenv("WHATSAPP_MCP_AUTH_TOKENS", "").strip()
    path_value = os.getenv("WHATSAPP_MCP_AUTH_TOKENS_FILE", "").strip()
    if path_value:
        path = Path(path_value).expanduser().resolve()
        mode = stat.S_IMODE(path.stat().st_mode)
        if mode & 0o077:
            raise ValueError(f"{path} must not be readable by group/others (chmod 600)")
        raw = path.read_text(encoding="utf-8")
    if not raw:
        return {}
    data = json.loads(raw)
    entries = data.get("tokens", data)
    if not isinstance(entries, dict):
        raise ValueError("auth tokens must be a JSON object")

    tokens: dict[str, dict[str, Any]] = {}
    for token, entry in entries.items():
        if not isinstance(token, str) or len(token) < 24:
            raise ValueError("each bearer token must have at least 24 characters")
        if not isinstance(entry, dict):
            raise ValueError("each token entry must be an object")
        tenant = whatsapp.validate_tenant_id(str(entry.get("tenant") or ""))
        api_token = str(entry.get("api_token") or "").strip()
        if not api_token:
            raise ValueError(f"token for tenant {tenant} is missing api_token")
        scopes = entry.get("scopes") or ["read"]
        if not isinstance(scopes, list) or not set(scopes).issubset(ALLOWED_SCOPES):
            raise ValueError(f"token for tenant {tenant} has invalid scopes")
        client_id = str(entry.get("client_id") or f"mcp-{tenant}")
        expires_at = entry.get("expires_at")
        if expires_at is not None:
            expires_at = int(expires_at)
        tokens[token] = {
            "tenant": tenant,
            "api_token": api_token,
            "scopes": list(dict.fromkeys(scopes)),
            "client_id": client_id,
            "expires_at": expires_at,
        }
    return tokens


class ScopedFastMCP(FastMCP):
    """FastMCP variant whose tools/list respects a read-only bearer scope."""

    async def list_tools(self):
        tools = await super().list_tools()
        try:
            token = get_access_token()
        except Exception:
            token = None
        if token is not None and "write" not in token.scopes:
            return [tool for tool in tools if tool.name not in WRITE_TOOL_NAMES]
        return tools


class IntrospectionTokenStore(TokenVerifier):
    """Validate opaque tokens against the panel and receive the tenant/key.

    The panel owns hashes and tenant credentials; this process never reads a
    token file in this mode.
    """

    def __init__(self, url: str, secret: str, ttl_seconds: float = 5.0):
        self._url = url
        self._secret = secret
        self._ttl = max(1.0, ttl_seconds)
        self._cache: dict[str, tuple[float, AccessToken | None]] = {}

    async def verify_token(self, token: str) -> AccessToken | None:
        now = time.monotonic()
        cached = self._cache.get(token)
        if cached and cached[0] > now:
            return cached[1]
        try:
            async with httpx.AsyncClient(timeout=5.0) as client:
                response = await client.post(
                    self._url,
                    headers={"Authorization": f"Bearer {self._secret}"},
                    json={"token": token},
                )
        except Exception:
            response = None
        access: AccessToken | None = None
        if response is not None and response.status_code == 200:
            try:
                data = response.json()
                tenant = whatsapp.validate_tenant_id(str(data["tenant"]))
                api_token = str(data["api_token"])
                scopes = [str(scope) for scope in data.get("scopes", ["read"])]
                if scopes and set(scopes).issubset(ALLOWED_SCOPES):
                    access = AccessToken(
                        token=token,
                        client_id=str(data.get("client_id") or f"mcp-{tenant}"),
                        scopes=scopes,
                        subject=str(data.get("subject") or tenant),
                        claims={"tenant": tenant, "api_token": api_token},
                    )
            except (KeyError, TypeError, ValueError, json.JSONDecodeError):
                access = None
        self._cache[token] = (now + self._ttl, access)
        return access


class TokenStore(TokenVerifier):
    def __init__(self, tokens: dict[str, dict[str, Any]]):
        self._tokens = tokens

    async def verify_token(self, token: str) -> AccessToken | None:
        entry = self._tokens.get(token)
        if entry is None:
            return None
        if entry["expires_at"] is not None and entry["expires_at"] < int(time.time()):
            return None
        return AccessToken(
            token=token,
            client_id=entry["client_id"],
            scopes=entry["scopes"],
            expires_at=entry["expires_at"],
            subject=entry["tenant"],
            claims={
                "tenant": entry["tenant"],
                "api_token": entry["api_token"],
            },
        )


def auth_settings() -> AuthSettings:
    base = public_url()
    return AuthSettings(
        issuer_url=base,
        resource_server_url=f"{base}{http_path()}",
        required_scopes=["read"],
        validate_token_resource=False,
    )


def transport_security() -> TransportSecuritySettings:
    configured = os.getenv("WHATSAPP_MCP_ALLOWED_HOSTS", "").strip()
    allowed_hosts = [item.strip() for item in configured.split(",") if item.strip()]
    if not allowed_hosts:
        from urllib.parse import urlparse
        public_host = urlparse(public_url()).hostname or "127.0.0.1"
        allowed_hosts = [f"{public_host}:*", "127.0.0.1:*", "localhost:*", "[::1]:*"]
    return TransportSecuritySettings(
        enable_dns_rebinding_protection=True,
        allowed_hosts=allowed_hosts,
        allowed_origins=[],
    )


def configured() -> bool:
    return bool(load_tokens())
