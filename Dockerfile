# Go build
FROM golang:1.25 AS go-builder
WORKDIR /src
COPY whatsapp-bridge/go.mod whatsapp-bridge/go.sum ./
RUN go mod download
COPY whatsapp-bridge/main.go ./
RUN CGO_ENABLED=1 go build -trimpath -ldflags="-s -w" -o /out/whatsapp-bridge .

# Python dependencies
FROM python:3.11-slim AS python-deps
COPY --from=ghcr.io/astral-sh/uv:latest /uv /bin/uv
WORKDIR /app/whatsapp-mcp-server
COPY whatsapp-mcp-server/pyproject.toml whatsapp-mcp-server/uv.lock ./
RUN uv sync --frozen --no-dev --no-install-project

# Minimal runtime: the MCP server is started on demand with docker exec.
FROM python:3.11-slim
ENV PATH="/app/whatsapp-mcp-server/.venv/bin:$PATH" \
    PYTHONUNBUFFERED=1
WORKDIR /app
COPY --from=go-builder /out/whatsapp-bridge /app/whatsapp-bridge/whatsapp-bridge
COPY --from=python-deps /app/whatsapp-mcp-server/.venv /app/whatsapp-mcp-server/.venv
COPY whatsapp-mcp-server/*.py /app/whatsapp-mcp-server/
COPY docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh
RUN chmod +x /usr/local/bin/docker-entrypoint.sh

VOLUME ["/app/whatsapp-bridge/store"]
EXPOSE 8080
ENTRYPOINT ["docker-entrypoint.sh"]
