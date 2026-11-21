# Deploy do painel na VPS

O workflow `.github/workflows/deploy-panel.yml` publica duas imagens no GHCR e faz o deploy por SSH. Nada é executado em produção sem um merge em `main` (ou `workflow_dispatch`).

## Topologia

```
Cloudflare Tunnel (dashboard)
  panel.isaacvshiarley.cloud ──► http://whatsapp-panel:3000   (rede default-app-network)
                                 │
                                 ├─ whatsapp-panel (Next.js)
                                 ├─ whatsapp-panel-db (Postgres, volume)
                                 └─ whatsapp-panel-migrate (one-shot)

BRIDGE_URL ──► bridge (VPS ou host local via túnel + Cloudflare Access)
```

- O painel não publica porta no host; só `expose: 3000` na rede Docker.
- O túnel `cloudflared` já está em `default-app-network`; o painel entra na mesma rede externa (`EDGE_NETWORK`).
- Postgres em volume próprio, sem porta pública.
- O bridge precisa ser acessível **do VPS**. Duas opções:
  1. hospedar o bridge na VPS junto do painel (mover `data/` exige backup e janela);
  2. expor o bridge atual por um túnel com Cloudflare Access e configurar `BRIDGE_CF_ACCESS_CLIENT_ID/SECRET` no painel.

## Pré-requisitos no GitHub

Secrets (Settings → Secrets and variables → Actions):

| Secret | Valor |
|---|---|
| `VPS_HOST` | `129.121.50.239` |
| `VPS_PORT` | `22022` |
| `VPS_USER` | `root` |
| `VPS_SSH_KEY` | chave privada de deploy (a pública em `/root/.ssh/authorized_keys`) |
| `GHCR_PULL_TOKEN` | PAT com `read:packages` para o VPS puxar as imagens |

Environment `production` com proteção (reviewers) recomendado.

## Pré-requisitos no VPS (uma vez)

```bash
mkdir -p /root/whatsapp-panel
cd /root/whatsapp-panel
# copie deploy/vps/.env.example para .env e preencha
chmod 600 .env

# túnel já existente em default-app-network; confirme:
docker network inspect default-app-network >/dev/null
```

No dashboard Cloudflare (tunnel token-based):
- Public hostname `panel.isaacvshiarley.cloud` → `http://whatsapp-panel:3000`
- Access policy para o painel (e-mail/OTP ou IdP).
- Para o bridge: hostname separado + Access service token; use o mesmo token no `.env`.

No Google Cloud:
- Authorized redirect URI: `https://panel.isaacvshiarley.cloud/api/auth/callback/google`

## Deploy

Automático após merge em `main` tocando `panel/**` ou `deploy/vps/**`. Manual: **Actions → Deploy panel → Run workflow**.

O job:
1. builda `whatsapp-panel:latest` e `whatsapp-panel-migrate:latest` (+ tag `sha-...`);
2. copia `deploy/vps/docker-compose.yml` para `/root/whatsapp-panel`;
3. `pull`, sobe `panel-db`, roda `panel-migrate`, sobe `panel`;
4. limpa imagens antigas (filtro de 7 dias).

## Rollback

As imagens têm tag `sha-<short>`. No VPS:

```bash
cd /root/whatsapp-panel
PANEL_IMAGE=ghcr.io/markhiarley/whatsapp-panel:sha-abc1234 \
PANEL_MIGRATE_IMAGE=ghcr.io/markhiarley/whatsapp-panel-migrate:sha-abc1234 \
docker compose up -d panel
```

Ou altere `PANEL_IMAGE` no `.env` e rode `docker compose up -d panel`.

## Operação e limites

- Disco da VPS estava em ~72% (27G livres); Postgres + imagens do painel cabem, mas monitore `docker system df`. Nunca `docker system prune --volumes`.
- Backup do painel:
  ```bash
  docker exec whatsapp-panel-db pg_dump -U panel panel > /root/whatsapp-panel/backup-$(date +%F).sql
  ```
- Logs: `docker logs -f whatsapp-panel`.
- Migração roda a cada deploy; Drizzle `push --force` aplica o schema atual. Faça backup antes de mudanças destrutivas.
