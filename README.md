# WhatsApp MCP Server

Connect WhatsApp to AI agents through the **Model Context Protocol (MCP)**.
This fork includes an updated Whatsmeow dependency, compatibility fixes, local timezone support, Docker, and an authenticated REST API.

## Docker (recommended)

Requirement: Docker with Compose.

```bash
printf 'WHATSAPP_API_TOKEN=%s\n' "$(openssl rand -hex 32)" > .env
docker compose up --build
```

On first use, scan the QR code shown in the terminal under **WhatsApp → Linked devices → Link a device**. The session is persisted in `./data`, so subsequent starts reconnect automatically.

The API is available only on the local machine at `http://127.0.0.1:8080`.

### Use an existing manual session

Stop the bridge first, then run:

```bash
mkdir -p data
cp whatsapp-bridge/store/{whatsapp,messages}.db data/
docker compose up --build
```

## MCP configuration

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

## REST API

Load the token without printing it:

```bash
set -a; . ./.env; set +a
```

Send a message:

```bash
curl -X POST http://127.0.0.1:8080/api/v1/send \
  -H "Authorization: Bearer $WHATSAPP_API_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"recipient":"5511999999999","message":"Hello!"}'
```

Download media:

```bash
curl -X POST http://127.0.0.1:8080/api/v1/download \
  -H "Authorization: Bearer $WHATSAPP_API_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"message_id":"...","chat_jid":"...@s.whatsapp.net"}'
```

The token is required, has no default value, and must remain only in the local `.env` file. API v1 responses use the `{success, data, error}` envelope documented in [`contracts/openapi.yaml`](contracts/openapi.yaml). The legacy `/api/send` and `/api/download` routes remain temporarily available for compatibility.

MCP tools return messages, chats, and contacts as structured JSON objects rather than preformatted text.

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
```

## Project structure

```text
whatsapp-mcp/
├── contracts/              # API v1 OpenAPI contract
├── whatsapp-bridge/
│   ├── main.go             # Initialization and lifecycle
│   ├── api.go              # REST API, contracts, and authentication
│   ├── handlers.go         # Events and message synchronization
│   ├── media.go            # Sending, downloading, and audio
│   └── store.go            # SQLite, contacts, chats, and messages
├── whatsapp-mcp-server/    # Python MCP server
├── docker-entrypoint.sh    # Bridge startup
├── Dockerfile              # Multi-stage build
├── docker-compose.yml
└── data/                   # Persistent session, ignored by Git
```

## Credits

Original project created by [Luke Harries (@lharries)](https://github.com/lharries): [lharries/whatsapp-mcp](https://github.com/lharries/whatsapp-mcp). This fork preserves the original project credits and MIT license.

## License

[MIT](LICENSE)
