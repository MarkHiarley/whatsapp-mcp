# PRODUCT.md

> Inferred from the explicit brief and the shipped bridge. Labeled assumptions, not an interview.

## O que é

Bridge WhatsApp multi-tenant (Go) + MCP server (Python) + painel web (Next.js). O painel é a superfície de operação: conectar números, ver conversas, acompanhar eventos, gerenciar regras, webhooks e credenciais.

## Quem usa

- **Operador/admin:** provisiona tenants, acompanha saúde, revoga credenciais.
- **Cliente (workspace):** pareia o próprio número, lê e envia mensagens, cria regras e webhooks, gerencia keys.
- **Agentes:** consomem eventos por cursor/webhook e usam o MCP; não usam o painel.

## Trabalhos principais

1. Parear um número e saber se a sessão está conectada.
2. Ler conversas e mensagens, com mídia, e responder.
3. Criar regras (grupo, número, texto, menção) e acompanhar eventos.
4. Configurar webhooks, ver entregas e diagnosticar falhas.
5. Emitir e revogar keys por integração.
6. Provisionar e remover tenants (admin).

## Modo

**Operate.** O usuário está no meio de uma tarefa; familiaridade, densidade e velocidade vencem expressão. A marca vive em detalhes precisos, não em decoração.

## Restrições

- Local-first, uma instância, API em `127.0.0.1`; o painel é a única entrada pública.
- Tokens de tenant nunca chegam ao browser; o BFF resolve credenciais no servidor.
- Dados reais do bridge; sem mock no produto.
- Fase atual: sem login; Better Auth + Postgres entram na fase seguinte.
- Sem páginas de marketing, sem app mobile.

## Não-objetivos

- Substituir o WhatsApp Web.
- Editor de fluxo/chatbot dentro do painel.
- Multi-região ou billing nesta fase.
