# ============================================================
# WhatsApp MCP Server — Docker
# Build:  docker build -t whatsapp-mcp .
# Run:    docker run -v ./data:/app/data -p 8080:8080 whatsapp-mcp
# ============================================================

# Stage 1: Build Go bridge
FROM golang:1.24 AS go-builder
WORKDIR /app
COPY whatsapp-bridge/ ./whatsapp-bridge/
RUN cd whatsapp-bridge && go build -o /whatsapp-bridge .

# Stage 2: Python MCP server
FROM python:3.11-slim
WORKDIR /app

# Install uv
COPY --from=ghcr.io/astral-sh/uv:latest /uv /uvx /bin/

# Copy Go bridge
COPY --from=go-builder /whatsapp-bridge /app/whatsapp-bridge/

# Copy Python server
COPY whatsapp-mcp-server/ /app/whatsapp-mcp-server/

# Install dependencies
WORKDIR /app/whatsapp-mcp-server
RUN uv sync --frozen

# Default command: start bridge + MCP server
EXPOSE 8080
CMD ["uv", "run", "python", "main.py", "--bridge-path", "/app/whatsapp-bridge", "--store-dir", "/app/data"]
