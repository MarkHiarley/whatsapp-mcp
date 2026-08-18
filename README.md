# WhatsApp MCP Server

Conecte seu WhatsApp a agentes de IA via **Model Context Protocol (MCP)**.
Fork com Whatsmeow atualizado, correções de compatibilidade, timezone local, Docker e API REST autenticada.

## Docker (recomendado)

Pré-requisito: Docker com Compose.

```bash
printf 'WHATSAPP_API_TOKEN=%s\n' "$(openssl rand -hex 32)" > .env
docker compose up --build
```

No primeiro uso, escaneie o QR Code exibido no terminal em **WhatsApp → Dispositivos conectados → Conectar dispositivo**. A sessão fica persistida em `./data`; os próximos inícios reconectam automaticamente.

A API fica disponível apenas na máquina local em `http://127.0.0.1:8080`.

### Usar uma sessão manual existente

Com o bridge parado:

```bash
mkdir -p data
cp whatsapp-bridge/store/{whatsapp,messages}.db data/
docker compose up --build
```

## Configuração MCP

O MCP usa `stdio` e é iniciado pelo cliente dentro do container já conectado:

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

O container deve estar ativo com `docker compose up -d` antes do cliente MCP iniciar.

## API REST

Carregue o token sem imprimi-lo:

```bash
set -a; . ./.env; set +a
```

Enviar mensagem:

```bash
curl -X POST http://127.0.0.1:8080/api/v1/send \
  -H "Authorization: Bearer $WHATSAPP_API_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"recipient":"5511999999999","message":"Olá!"}'
```

Baixar mídia:

```bash
curl -X POST http://127.0.0.1:8080/api/v1/download \
  -H "Authorization: Bearer $WHATSAPP_API_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"message_id":"...","chat_jid":"...@s.whatsapp.net"}'
```

O token é obrigatório, não possui valor padrão e deve permanecer somente no `.env` local. As respostas da API v1 usam o envelope `{success, data, error}` documentado em [`contracts/openapi.yaml`](contracts/openapi.yaml). As rotas antigas `/api/send` e `/api/download` permanecem temporariamente disponíveis para compatibilidade.

As ferramentas MCP retornam mensagens, chats e contatos como objetos JSON estruturados, não como texto pré-formatado.

## Execução manual

Pré-requisitos: Go 1.25+, Python 3.11+ e UV.

```bash
export WHATSAPP_API_TOKEN="$(openssl rand -hex 32)"
(cd whatsapp-bridge && go run .)
```

Em outro terminal:

```bash
cd whatsapp-mcp-server
uv sync
uv run python main.py
```

## Testes

```bash
cd whatsapp-bridge && go test ./...
cd ../whatsapp-mcp-server && uv run python test_main.py
```

## Estrutura

```text
whatsapp-mcp/
├── contracts/              # Contrato OpenAPI da API v1
├── whatsapp-bridge/        # Bridge Go, REST e conexão WhatsApp
├── whatsapp-mcp-server/    # Servidor MCP Python
├── docker-entrypoint.sh    # Inicialização do bridge
├── Dockerfile              # Build multi-stage
├── docker-compose.yml
└── data/                   # Sessão persistente, ignorada pelo Git
```

## Créditos

Projeto original criado por [Luke Harries (@lharries)](https://github.com/lharries): [lharries/whatsapp-mcp](https://github.com/lharries/whatsapp-mcp). Este fork mantém os créditos e a licença MIT do projeto original.

## Licença

[MIT](LICENSE)
