# WhatsApp MCP Server

Connect WhatsApp to AI agents through the **Model Context Protocol (MCP)**.
This fork includes an updated Whatsmeow dependency, compatibility fixes, local timezone support, Docker, an authenticated REST API, and a multi-tenant design (one process, many WhatsApp numbers) with durable events and webhooks.

> **Implementation status.** This branch freezes the multi-tenant and event contracts: [`contracts/openapi.yaml`](contracts/openapi.yaml) (source of truth for every route), [`contracts/tenants.example.json`](contracts/tenants.example.json), `whatsapp-bridge/tenant_contract.go`, and `whatsapp-bridge/event_contract.go`.
> The running bridge currently serves only the legacy routes on the `default` tenant (`/api/v1/send`, `/api/v1/download`, `/api/send`, `/api/download`). The tenant, key, QR, rule, event, and webhook routes described below are the target contract and are not served yet. The MCP per-tenant setup depends on the same work.

## Docker (recommended)

Requirement: Docker with Compose.

```bash
printf 'WHATSAPP_API_TOKEN=%s\n' "$(openssl rand -hex 32)" > .env
docker compose up --build
```

On first use, scan the QR code shown in the terminal under **WhatsApp → Linked devices → Link a device**. The session is persisted in `./data`, so subsequent starts reconnect automatically.

The API is available only on the local machine at `http://127.0.0.1:8080`.

To enable tenant provisioning, also set `WHATSAPP_ADMIN_TOKEN` in `.env` (see [Environment variables](#environment-variables)).

### Use an existing manual session

Stop the bridge first, then run:

```bash
mkdir -p data
cp whatsapp-bridge/store/{whatsapp,messages}.db data/
docker compose up --build
```

## Multi-tenant

### Overview

- **One process, N numbers.** Each tenant is one WhatsApp account.
- **One client and one database per tenant.** Each tenant has its own whatsmeow session (`whatsapp.db`) and its own message store (`messages.db`). Tenants never share state.
- **The `default` tenant keeps the legacy paths.** Its files stay at `store/whatsapp.db` and `store/messages.db`, so existing installs work without migration.
- **Tenant IDs** match `^[a-z0-9][a-z0-9-]{1,31}$` (for example, `cliente-x`).

### Layout

```text
store/tenants.json                         # tenants and key hashes (never raw tokens)
store/whatsapp.db + store/messages.db      # default
store/tenants/<id>/whatsapp.db
store/tenants/<id>/messages.db
store/tenants/<id>/media/<chat_jid>/
```

Paths are relative to the store directory. In Docker, the host's `./data` is mounted at `/app/whatsapp-bridge/store`, so `data/tenants/cliente-x/messages.db` on the host is `store/tenants/cliente-x/messages.db` inside the container.

### Environment variables

| Variable | Used by | Required | Description | Status in this branch |
|---|---|---|---|---|
| `WHATSAPP_API_TOKEN` | bridge, MCP | Yes | Key of the `default` tenant. Also the MCP's credential when it points at the default tenant. | Implemented |
| `WHATSAPP_ADMIN_TOKEN` | bridge | No | Admin token for provisioning (`/api/v1/tenants`) and for managing keys and webhooks of any tenant. If unset, `/tenants` routes return `503`. | Contract |
| `WHATSAPP_STORE_DIR` | bridge | No | Base directory for `tenants.json`, databases, and media. | Contract |
| `WHATSAPP_TENANT_ID` | MCP | No | Tenant whose `whatsapp.db` and `messages.db` the MCP reads. | Contract |
| `WHATSAPP_API_BASE_URL` | MCP | No | Base URL of the bridge API the MCP calls. | Contract |
| `TZ` | bridge, MCP | No | Timezone for timestamps. Compose sets `America/Sao_Paulo`. | Implemented |
| `PORT` | bridge | No | HTTP port of the API. The container listens on `8080`, published on `127.0.0.1:8080`. | Contract |

Generate both tokens with `openssl rand -hex 32`. Keep them in the local `.env` file only.

### Security model

- **The admin token is the trust root.** It creates and deletes tenants, and issues and revokes keys and webhooks for any tenant. Never give it to a client. Clients receive a tenant key.
- **Keys are per tenant.** A tenant can have several keys, each with a label. A key authenticates requests and resolves to exactly one tenant.
- **Keys are stored hashed.** The bridge stores the SHA-256 hash of each token in `tenants.json`. The raw token appears only once, in the response that creates the key.
- **Revocation is immediate.** A revoked key stops working on the next request. Each key reports `last_used_at`, so you can see whether it is still in use.
- **Tenants only reach their own paths.** Databases and media are scoped to the tenant that owns the key.
- **Cross-tenant access returns 403.** The `{tenant}` in a path is only a check against the key's tenant. A key used on another tenant's path gets `403 FORBIDDEN`.

### Operational flow

The examples use `curl` against the local API. Load the admin token without printing it:

```bash
set -a; . ./.env; set +a
BASE=http://127.0.0.1:8080/api/v1
```

**1. Create a tenant and keep its first key.** The token is returned once, so the command below writes it to a `0600` file outside the repository without printing it. Run it once: creating the same `id` again returns `409`.

```bash
mkdir -p "$HOME/.config/whatsapp-mcp" && chmod 700 "$HOME/.config/whatsapp-mcp"
curl -sS -X POST "$BASE/tenants" \
  -H "Authorization: Bearer $WHATSAPP_ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"id":"cliente-x","label":"Cliente X"}' \
  | jq -r '.data.token' > "$HOME/.config/whatsapp-mcp/cliente-x.token"
chmod 600 "$HOME/.config/whatsapp-mcp/cliente-x.token"
TENANT_TOKEN="$(cat "$HOME/.config/whatsapp-mcp/cliente-x.token")"
```

The examples use `jq`. In production, move the token to your secret manager.

**2. Pair the number.** Open the QR stream and scan the code with the phone (**WhatsApp → Linked devices → Link a device**). The stream emits `qr` events with the code and `status` events (`pairing`, `connected`, `logged_out`), and closes when pairing finishes.

```bash
curl -N -sS "$BASE/tenants/cliente-x/qr" \
  -H "Authorization: Bearer $TENANT_TOKEN"
```

**3. Check the session.**

```bash
curl -sS "$BASE/tenants/cliente-x/status" \
  -H "Authorization: Bearer $TENANT_TOKEN" | jq '.data.tenant.status'
```

**4. Send a message.** Use `message` for text or `media_path` for a file. `media_path` must be a path the bridge can read inside the container.

```bash
curl -sS -X POST "$BASE/tenants/cliente-x/send" \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"recipient":"5511999999999","message":"Olá!"}'
```

**5. Download media** from a stored message. The response includes the local `path`.

```bash
curl -sS -X POST "$BASE/tenants/cliente-x/download" \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"message_id":"...","chat_jid":"...@s.whatsapp.net"}'
```

**6. Manage keys** (admin token). Issue a key for each client application or machine so they can be revoked separately.

```bash
# List keys (never shows the token)
curl -sS "$BASE/tenants/cliente-x/keys" \
  -H "Authorization: Bearer $WHATSAPP_ADMIN_TOKEN"

# Issue a key (the token is returned only once)
curl -sS -X POST "$BASE/tenants/cliente-x/keys" \
  -H "Authorization: Bearer $WHATSAPP_ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"label":"laptop-ana"}'

# Revoke a key by its id (key_...)
curl -sS -X DELETE "$BASE/tenants/cliente-x/keys/key_0123456789abcdef" \
  -H "Authorization: Bearer $WHATSAPP_ADMIN_TOKEN"
```

**7. Create an event rule and read events by cursor.** Rules are evaluated for every incoming message. Matching messages become events with a sequence number (`seq`).

```bash
curl -sS -X POST "$BASE/tenants/cliente-x/rules" \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
        "id": "urgentes-grupo",
        "name": "Urgentes no grupo",
        "match": {
          "is_group": true,
          "text_contains": ["urgente", "socorro"]
        }
      }'
```

Match fields are combined with AND. Values inside a list are combined with OR:

| Field | Matches |
|---|---|
| `chat_jids` | Group (`...@g.us`, whole JID) or direct chat |
| `sender_jids` | Sender. A bare phone number (`5511999999999`) matches with or without `@s.whatsapp.net` |
| `is_group` | Group (`true`) or direct chat (`false`) |
| `is_from_me` | Messages sent by the tenant's own number |
| `mentioned_me` | Messages that mention the tenant's number |
| `text_contains` | Case-insensitive keyword |
| `text_regex` | Regular expression on the text |

Read events. Without `after`, `consumer` resumes from the consumer's saved cursor. Then pass `next_after` back as `after`:

```bash
curl -sS "$BASE/tenants/cliente-x/events?rule=urgentes-grupo&consumer=crm&limit=100" \
  -H "Authorization: Bearer $TENANT_TOKEN" | jq '.data'
# .data.events: array ordered by seq; .data.next_after: cursor for the next call

curl -sS "$BASE/tenants/cliente-x/events?rule=urgentes-grupo&after=42&limit=100" \
  -H "Authorization: Bearer $TENANT_TOKEN"
```

Enable, disable, or replace a rule's match with `PATCH .../rules/{ruleId}` (for example, `{"enabled":false}`), and remove it with `DELETE .../rules/{ruleId}`.

Acknowledge what you processed. Delivery is at-least-once, so process idempotently (by `message_id` or `seq`). Ack only after processing:

```bash
curl -sS -X POST "$BASE/tenants/cliente-x/consumers/crm/ack" \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"rule_id":"urgentes-grupo","seq":57}'
```

**8. Create a webhook and verify signatures** (admin token). The secret appears only in the create response:

```bash
curl -sS -X POST "$BASE/tenants/cliente-x/webhooks" \
  -H "Authorization: Bearer $WHATSAPP_ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"url":"https://hooks.example.com/whatsapp","rule_id":"urgentes-grupo"}'
```

Each delivery carries:

- `X-WhatsApp-Signature: sha256=<hex>`: HMAC-SHA256 of the raw request body, keyed with the webhook secret.
- `X-WhatsApp-Event-Id: <tenant>:<seq>`: unique per event. Use it to deduplicate retries.

Verify the signature over the raw bytes, before parsing JSON:

```python
import hashlib
import hmac


def verify_signature(secret: str, raw_body: bytes, signature_header: str) -> bool:
    expected = "sha256=" + hmac.new(secret.encode(), raw_body, hashlib.sha256).hexdigest()
    return hmac.compare_digest(expected, signature_header)
```

Other webhook operations (admin token): list `GET .../webhooks`, update `PATCH .../webhooks/{webhookId}` (URL, rule, `enabled`), delete `DELETE .../webhooks/{webhookId}`, rotate the secret `POST .../webhooks/{webhookId}/secret` (the new secret is shown once), and inspect attempts with `GET .../webhooks/{webhookId}/deliveries?limit=50`.

The full list of routes, request bodies, and response schemas is in [`contracts/openapi.yaml`](contracts/openapi.yaml).

### MCP per tenant

Run one MCP server entry per tenant. Each entry uses a tenant key and sets `WHATSAPP_TENANT_ID`:

```json
{
  "mcpServers": {
    "whatsapp-cliente-x": {
      "command": "docker",
      "args": ["exec", "-i", "-w", "/app/whatsapp-mcp-server",
               "-e", "WHATSAPP_TENANT_ID=cliente-x",
               "-e", "WHATSAPP_API_TOKEN=<key-do-tenant>",
               "whatsapp-mcp", "python", "main.py"]
    }
  }
}
```

The 12 existing tools are unchanged: `search_contacts`, `list_messages`, `list_chats`, `get_chat`, `get_direct_chat_by_contact`, `get_contact_chats`, `get_last_interaction`, `get_message_context`, `send_message`, `send_file`, `send_audio_message`, and `download_media`. Their arguments and return shapes do not change.

`WHATSAPP_TENANT_ID` selects which database the server reads (`store/tenants/<id>/messages.db`). `WHATSAPP_API_TOKEN` is the tenant key used for sending and downloading. Keep both values for the same tenant.

The `-w /app/whatsapp-mcp-server` argument is required so that `main.py` resolves its imports, as in the legacy configuration below.

### Legacy routes

The `default` tenant keeps the routes that existed before multi-tenancy. They work with `WHATSAPP_API_TOKEN`:

- `POST /api/v1/send` and `POST /api/v1/download` (same bodies as the tenant routes)
- `POST /api/send` and `POST /api/download` (pre-v1 routes, not in the OpenAPI)

The OpenAPI also lists `/send` and `/download` (under `/api/v1`) as deprecated aliases for the default tenant. Migrate to `/tenants/{tenant}/send` and `/tenants/{tenant}/download`.

```bash
set -a; . ./.env; set +a
curl -sS -X POST http://127.0.0.1:8080/api/v1/send \
  -H "Authorization: Bearer $WHATSAPP_API_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"recipient":"5511999999999","message":"Hello!"}'
```

### Failure modes

Errors use the `{success, data, error}` envelope, with `error.code` and `error.message`.

| HTTP | Code | When |
|---|---|---|
| 400 | `INVALID_REQUEST`, `RULE_INVALID`, `WEBHOOK_INVALID` | Malformed body, unknown field, invalid regex or empty match value |
| 401 | `UNAUTHORIZED` | Key missing, invalid, or revoked |
| 403 | `FORBIDDEN` | Key belongs to another tenant, or the admin token is required and missing |
| 404 | `NOT_FOUND` | Tenant, key, rule, or webhook does not exist |
| 405 | `METHOD_NOT_ALLOWED` | Wrong HTTP method |
| 409 | `CONFLICT` | The `id` (tenant, rule, or key) already exists |
| 500 | `SEND_FAILED`, `DOWNLOAD_FAILED` | WhatsApp rejected the operation |
| 503 | `TENANT_DISCONNECTED` | The tenant's session is not connected |
| 503 | `ADMIN_DISABLED` | `WHATSAPP_ADMIN_TOKEN` is not set; `/tenants` routes are unavailable |

### Media

- **Local path per tenant.** Downloaded files are saved under the tenant's media directory: `store/tenants/<id>/media/<chat_jid>/`. The `path` in the response is the local path inside the container. The default tenant uses `store/<chat_jid>/` (with `:` replaced by `_`).
- **Local cache.** If the file already exists locally, the download returns it without contacting WhatsApp's CDN.
- **CDN query string.** The CDN URL carries signed parameters (`ccb`, `oh`, `oe`). Dropping them makes the CDN return `403`, so the bridge keeps the full query string when it downloads.

## REST API

Load the token without printing it:

```bash
set -a; . ./.env; set +a
```

Send a message on the default tenant:

```bash
curl -X POST http://127.0.0.1:8080/api/v1/send \
  -H "Authorization: Bearer $WHATSAPP_API_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"recipient":"5511999999999","message":"Hello!"}'
```

Download media on the default tenant:

```bash
curl -X POST http://127.0.0.1:8080/api/v1/download \
  -H "Authorization: Bearer $WHATSAPP_API_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"message_id":"...","chat_jid":"...@s.whatsapp.net"}'
```

The token is required, has no default value, and must remain only in the local `.env` file. API v1 responses use the `{success, data, error}` envelope documented in [`contracts/openapi.yaml`](contracts/openapi.yaml).

MCP tools return messages, chats, and contacts as structured JSON objects rather than preformatted text.

## MCP configuration (default tenant)

The MCP server uses `stdio` and is started by the client inside the connected container:

```json
{
  "mcpServers": {
    "whatsapp": {
      "command": "docker",
      "args": [
        "exec", "-i",
        "-w", "/app/whatsapp-mcp-server",
        "whatsapp-mcp",
        "python", "main.py"
      ]
    }
  }
}
```

The container must be running with `docker compose up -d` before the MCP client starts.

## Manual setup

Requirements: Go 1.25+, Python 3.11+, and UV.

```bash
export WHATSAPP_API_TOKEN="$(openssl rand -hex 32)"
(cd whatsapp-bridge && go run .)
```

In another terminal:

```bash
cd whatsapp-mcp-server
uv sync
uv run python main.py
```

## Tests

```bash
cd whatsapp-bridge && go test ./...
cd ../whatsapp-mcp-server && uv run python test_main.py
cd .. && uv run --with pyyaml python -c "import yaml; yaml.safe_load(open('contracts/openapi.yaml'))"
```

CI runs the same checks, plus `go vet` and `mypy`.

## Project structure

```text
whatsapp-mcp/
├── contracts/
│   ├── openapi.yaml           # API v1 OpenAPI contract (source of truth for routes)
│   └── tenants.example.json   # Example of store/tenants.json (hashes only)
├── whatsapp-bridge/
│   ├── main.go                # Initialization and lifecycle
│   ├── api.go                 # REST API, contracts, and authentication
│   ├── tenant_contract.go     # Tenant, key, and hashing contract
│   ├── event_contract.go      # Event, rule, and event-schema contract
│   ├── handlers.go            # Events and message synchronization
│   ├── media.go               # Sending, downloading, and audio
│   └── store.go               # SQLite, contacts, chats, and messages
├── whatsapp-mcp-server/       # Python MCP server
├── docker-entrypoint.sh       # Bridge startup
├── Dockerfile                 # Multi-stage build
├── docker-compose.yml
└── data/                      # Persistent session, ignored by Git
```

## Credits

Original project created by [Luke Harries (@lharries)](https://github.com/lharries): [lharries/whatsapp-mcp](https://github.com/lharries/whatsapp-mcp). This fork preserves the original project credits and MIT license.

## License

[MIT](LICENSE)
