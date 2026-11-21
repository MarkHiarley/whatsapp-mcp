# Deploy do stack na VPS

O workflow `.github/workflows/deploy-panel.yml` publica três imagens no GHCR (bridge, painel e migrações) e faz o deploy por SSH. Nada roda em produção sem um merge em `main` (ou `workflow_dispatch`).

## Topologia

```
Cloudflare Tunnel (dashboard)
  panel.isaacvshiarley.cloud ──► http://whatsapp-panel:3000   (default-app-network)

stack whatsapp-panel
  ├─ whatsapp-mcp        → bridge Go + sessão em /root/whatsapp-mcp/data
  ├─ whatsapp-panel      → Next.js, sem porta pública
  ├─ whatsapp-panel-db   → Postgres, volume próprio
  └─ whatsapp-panel-migrate (one-shot)
```

- O painel e o bridge ficam na rede default do compose; o painel chama `http://whatsapp-mcp:8080/api/v1`.
- Só o painel entra na rede externa `default-app-network`, a mesma dos túneis.
- Nenhuma porta do painel/bridge é publicada no host.

## Pré-requisitos no GitHub

Secrets (Settings → Secrets and variables → Actions):

| Secret | Valor |
|---|---|
| `VPS_HOST` | `129.121.50.239` |
| `VPS_PORT` | `22022` |
| `VPS_USER` | `root` |
| `VPS_SSH_KEY` | chave privada de deploy (a pública em `/root/.ssh/authorized_keys`) |

As imagens no GHCR são privadas e o VPS faz o pull com o `GITHUB_TOKEN` do próprio workflow (válido durante o job). Não é necessário PAT de longa duração. Se o pacote for tornado público, o login pode ser removido.

Environment `production` com proteção (reviewers) recomendado.

## Pré-requisitos no VPS (uma vez)

```bash
mkdir -p /root/whatsapp-panel /root/whatsapp-mcp
cd /root/whatsapp-panel
cp deploy/vps/.env.example .env   # preencha e chmod 600
```

O `.env` precisa de:
- `PANEL_DB_PASSWORD`, `BETTER_AUTH_SECRET`, `PANEL_ENCRYPTION_KEY`
- `BETTER_AUTH_URL=https://panel.isaacvshiarley.cloud`
- `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`
- `PRIMER_ADMIN`, `PANEL_ADMIN_EMAILS`
- `WHATSAPP_API_TOKEN` (a mesma key da sessão em `/root/whatsapp-mcp/data/tenants.json`)
- `WHATSAPP_ADMIN_TOKEN` e `BRIDGE_ADMIN_TOKEN` com o **mesmo** valor

Copie a sessão para `/root/whatsapp-mcp/data` com o bridge local **parado** (nunca dois clients na mesma sessão):

```bash
rsync -a ./data/ vps:/root/whatsapp-mcp/data/
```

No dashboard Cloudflare (tunnel token-based):
- Public hostname `panel.isaacvshiarley.cloud` → `http://whatsapp-panel:3000`
- Access policy para o painel (e-mail/OTP ou IdP).

No Google Cloud:
- Authorized redirect URI: `https://panel.isaacvshiarley.cloud/api/auth/callback/google`

## Bootstrap manual (antes do primeiro CD)

Se for subir o stack sem esperar o workflow, com imagens locais (`whatsapp-bridge:local`, `whatsapp-panel:local`):

```bash
cd /root/whatsapp-panel
docker compose up -d --wait panel-db
docker exec -i whatsapp-panel-db psql -U panel -d panel < drizzle/0000_watery_magik.sql
docker compose up -d whatsapp-mcp
docker compose up -d --no-deps panel
```

O painel standalone precisa de `HOSTNAME=0.0.0.0` (já no compose) para o túnel alcançar o container. Depois do primeiro CD, as imagens do GHCR assumem via export no workflow.

## Deploy

Automático após merge em `main` tocando `panel/**`, `whatsapp-bridge/**` ou `deploy/vps/**`. Manual: **Actions → Deploy panel → Run workflow**.

O job:
1. builda `whatsapp-bridge`, `whatsapp-panel` e `whatsapp-panel-migrate` (+ tag `sha-...`);
2. copia `deploy/vps/docker-compose.yml` para `/root/whatsapp-panel`;
3. `pull`, sobe `panel-db`, roda `panel-migrate`, sobe `whatsapp-mcp` e `panel`;
4. limpa imagens antigas (filtro de 7 dias).

## Rollback

As imagens têm tag `sha-<short>`. No VPS:

```bash
cd /root/whatsapp-panel
PANEL_IMAGE=ghcr.io/markhiarley/whatsapp-panel:sha-abc1234 \
PANEL_MIGRATE_IMAGE=ghcr.io/markhiarley/whatsapp-panel-migrate:sha-abc1234 \
BRIDGE_IMAGE=ghcr.io/markhiarley/whatsapp-bridge:sha-abc1234 \
docker compose up -d panel whatsapp-mcp
```

Ou altere as variáveis no `.env` e rode `docker compose up -d panel whatsapp-mcp`.

## Operação e limites

- Disco da VPS estava em ~72% (27G livres); bridge + painel + Postgres cabem, mas monitore `docker system df`. Nunca `docker system prune --volumes`.
- Backup do painel:
  ```bash
  docker exec whatsapp-panel-db pg_dump -U panel panel > /root/whatsapp-panel/backup-$(date +%F).sql
  ```
- Backup da sessão (com o bridge parado): copie `/root/whatsapp-mcp/data`.
- Logs: `docker logs -f whatsapp-panel` e `docker logs -f whatsapp-mcp`.
- Migração roda a cada deploy; Drizzle `push --force` aplica o schema atual. Faça backup antes de mudanças destrutivas.
