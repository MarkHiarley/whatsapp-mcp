import { headers } from "next/headers";
import { connection } from "next/server";
import { Plug, ShieldCheck } from "lucide-react";

import { McpTokenCard } from "@/components/mcp-token-card";
import { McpTokenList } from "@/components/mcp-token-list";
import { PageHeader } from "@/components/page-header";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { listMcpTokens } from "@/lib/mcp-tokens";
import { requireTenantContext } from "@/lib/session";

export const instant = false;

async function endpointUrl(): Promise<string | null> {
  const configured = process.env.MCP_PUBLIC_URL?.trim().replace(/\/+$/, "");
  if (configured) {
    return configured.endsWith("/api/mcp") ? configured : `${configured}/api/mcp`;
  }
  const requestHeaders = await headers();
  const host = requestHeaders.get("x-forwarded-host") ?? requestHeaders.get("host");
  if (!host) {
    return null;
  }
  const proto = requestHeaders.get("x-forwarded-proto") ?? (host.startsWith("localhost") ? "http" : "https");
  return `${proto}://${host}/api/mcp`;
}

export default async function McpPage() {
  await connection();
  const context = await requireTenantContext();

  const endpoint = await endpointUrl();
  const configured = Boolean(endpoint && process.env.MCP_INTERNAL_URL);
  const canManage = context.role === "admin" || context.role === "owner";
  const tokens = canManage
    ? (await listMcpTokens(context.organizationId, context.tenantId)).map((token) => ({
        id: token.id,
        name: token.name,
        scopes: token.scopes,
        tenantId: token.tenantId,
        createdAt: token.createdAt.toISOString(),
        lastUsedAt: token.lastUsedAt?.toISOString() ?? null,
        revokedAt: token.revokedAt?.toISOString() ?? null,
      }))
    : [];

  return (
    <>
      <PageHeader
        title="MCP remoto"
        description={
          configured
            ? "O endpoint já está no ar. Gere um token por agente e cole a configuração no seu cliente MCP."
            : "O endpoint será ativado quando o administrador configurar este ambiente."
        }
      />

      <div className="grid gap-4 xl:grid-cols-[1.15fr_0.85fr]">
        <div className="space-y-4">
          <Card>
            <CardHeader className="border-b">
              <div className="flex items-start justify-between gap-3">
                <div className="space-y-1">
                  <CardTitle className="flex items-center gap-2 text-sm">
                    <Plug className="size-4 text-primary" aria-hidden />
                    Endpoint
                  </CardTitle>
                  <CardDescription>Use esta URL no cliente MCP, com o token gerado abaixo.</CardDescription>
                </div>
                <Badge variant={configured ? "default" : "outline"}>{configured ? "ativo" : "pendente"}</Badge>
              </div>
            </CardHeader>
            <CardContent className="pt-4">
              {endpoint ? (
                <code className="block overflow-x-auto rounded-md border bg-muted/50 px-3 py-2 font-mono text-xs">
                  {endpoint}
                </code>
              ) : (
                <p className="rounded-md border border-dashed bg-muted/30 p-3 text-sm text-muted-foreground">
                  O endpoint ainda não foi configurado neste ambiente.
                </p>
              )}
            </CardContent>
          </Card>

          {canManage ? (
            <>
              <McpTokenCard endpoint={configured ? endpoint : null} tenantId={context.tenantId} />
              <McpTokenList tokens={tokens} />
            </>
          ) : (
            <Card>
              <CardContent className="p-4 text-sm text-muted-foreground">
                Apenas owner ou admin do tenant podem gerar tokens.
              </CardContent>
            </Card>
          )}
        </div>

        <div className="space-y-4">
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-sm">
                <ShieldCheck className="size-4 text-primary" aria-hidden />
                Como funciona
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-3 text-sm text-muted-foreground">
              <p>O token é vinculado ao tenant da sessão. Clientes não recebem a key do bridge.</p>
              <p>
                Tokens <code className="font-mono">read</code> só listam e leem;{" "}
                <code className="font-mono">write</code> adiciona drafts, envio e ack.
              </p>
              <p>O valor cru do token aparece uma única vez. A revogação fica na API administrativa de tokens.</p>
              <p>O proxy do painel fala com o MCP pela rede interna; nada de porta pública direta.</p>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle className="text-sm">Teste rápido</CardTitle>
            </CardHeader>
            <CardContent className="space-y-2 text-sm text-muted-foreground">
              <p>Sem token, o endpoint responde <code className="font-mono">401</code>:</p>
              <pre className="overflow-x-auto rounded-md border bg-muted/50 p-3 font-mono text-[11px]">
                {`curl -i ${endpoint ?? "<endpoint>"}`}
              </pre>
              <p>Com token, o cliente faz o handshake MCP e lista apenas os tools do escopo.</p>
            </CardContent>
          </Card>
        </div>
      </div>
    </>
  );
}
