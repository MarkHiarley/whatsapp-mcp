# WhatsApp MCP Server

Conecte seu WhatsApp a agentes de IA via **Model Context Protocol (MCP)**.
Fork com correções e melhorias do [lharries/whatsapp-mcp](https://github.com/lharries/whatsapp-mcp).

## ✨ Melhorias deste fork

- ✅ **Whatsmeow atualizado** (sem erro 405 "Client outdated")
- ✅ **Breaking fixes**: `context.Context` adicionado em 5 funções
- ✅ **Timezone**: mensagens exibem fuso horário local (ex: `2026-07-23 04:05:36 BRT`)
- ✅ **Docker**: roda sem instalar Go ou Python
- ✅ **CI**: GitHub Actions (build + lint)
- ✅ **API REST** na porta 8080 (`POST /api/send`)

## 🚀 Como usar

### Opção 1: Docker (recomendado)

```bash
docker compose up
```

Escaneie o QR Code que aparece no terminal com seu WhatsApp
(Configurações > Dispositivos conectados > Conectar um dispositivo).

### Opção 2: Manual

Pré-requisitos: Go 1.24+, Python 3.11+, UV

```bash
# Build bridge Go
cd whatsapp-bridge && go build -o whatsapp-bridge .

# Instalar server Python
cd ../whatsapp-mcp-server && uv sync

# Rodar
uv run python main.py --bridge-path ../whatsapp-bridge/whatsapp-bridge --store-dir ./data
```

### Opção 3: API REST direta

O bridge também expõe uma API HTTP na porta 8080:

```bash
# Enviar mensagem
curl -X POST http://localhost:8080/api/send \
  -H "Content-Type: application/json" \
  -d '{"recipient":"5511999999999","message":"Olá!"}'

# Baixar mídia
curl -X POST http://localhost:8080/api/download \
  -H "Content-Type: application/json" \
  -d '{"messageId":"...","chatJid":"...@s.whatsapp.net"}'
```

## 🔧 Configuração no Pi / Claude Desktop

Adicione no `mcp.json`:

```json
{
  "mcpServers": {
    "whatsapp": {
      "command": "uv",
      "args": [
        "--directory",
        "/caminho/whatsapp-mcp/whatsapp-mcp-server",
        "run",
        "main.py",
        "--bridge-path",
        "/caminho/whatsapp-mcp/whatsapp-bridge/whatsapp-bridge",
        "--store-dir",
        "/caminho/whatsapp-mcp/data"
      ]
    }
  }
}
```

## 🧪 Testes

```bash
cd whatsapp-mcp-server && uv run python test_main.py
```

## 📦 Estrutura

```
whatsapp-mcp/
├── whatsapp-bridge/        # Bridge Go (conexão WhatsApp)
│   └── main.go
├── whatsapp-mcp-server/    # Servidor Python MCP
│   ├── main.py
│   ├── whatsapp.py
│   └── test_main.py
├── Dockerfile
├── docker-compose.yml
└── .github/workflows/ci.yml
```

## 📄 Licença

MIT
