import { connection } from "next/server";
import { Plug, ShieldCheck, Terminal } from "lucide-react";

import { PageHeader } from "@/components/page-header";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { requireTenantContext } from "@/lib/session";

export const instant = false;

function endpointUrl(): string | null {
  const base = process.env.MCP_PUBLIC_URL?.trim().replace(/\/+$/, "");
  if (!base) {
    return null;
  }
  return base.endsWith("/mcp") ? base : `${base}/mcp`;
}

const SERVER_ENV = `WHATSAPP_MCP_TRANSPORT=streamable-http
WHATSAPP_MCP_HTTP_HOST=127.0.0.1
WHATSAPP_MCP_HTTP_PORT=8765
WHATSAPP_MCP_PUBLIC_URL=https://mcp.example.com
WHATSAPP_MCP_AUTH_TOKENS_FILE=/etc/whatsapp-mcp/tokens.json`;

const TOKENS_JSON = `{
  "tokens": {
    "<bearer-token-do-agente>": {
      "tenant": "cliente-x",
      "api_token": "<tenant-key-do-bridge>",
      "scopes": ["read"],
      "client_id": "agent-read"
    }
  }
}`;

const CURSOR_JSON = `{
  "mcpServers": {
    "whatsapp": {
      "url": "<ENDPOINT>",
      "headers": {
        "Authorization": "Bearer <TOKEN>"
      }
    }
  }
}`;

export default async function McpPage() {
  await connection();
  await requireTenantContext();

  const endpoint = endpointUrl();
  const endpointLabel = endpoint ?? "https://mcp.seudominio.com/mcp";
  const claudeCode = `claude mcp add --transport http whatsapp \\
  ${endpointLabel} \\
  --header "Authorization: Bearer <TOKEN>"`;

  return (
    <>
      <PageHeader
        title="MCP remoto"
        description="Conecte agentes ao WhatsApp por Streamable HTTP, com token por tenant e escopo de leitura/escrita."
      />

      <div className="grid gap-4 xl:grid-cols-[1.15fr_0.85fr]">
        <Card>
          <CardHeader className="border-b">
            <div className="flex items-start justify-between gap-3">
              <div className="space-y-1">
                <CardTitle className="flex items-center gap-2 text-sm">
                  <Plug className="size-4 text-primary" aria-hidden />
                  Endpoint
                </CardTitle>
                <CardDescription>O servidor MCP é separado do painel; esta página é só a documentação.</CardDescription>
              </div>
              <Badge variant={endpoint ? "default" : "outline"}>{endpoint ? "configurado" : "pendente"}</Badge>
            </div>
          </CardHeader>
          <CardContent className="space-y-4 pt-4">
            {endpoint ? (
              <div className="space-y-1.5">
                <p className="text-xs font-medium text-muted-foreground">URL do endpoint</p>
                <code className="block overflow-x-auto rounded-md border bg-muted/50 px-3 py-2 font-mono text-xs">
                  {endpoint}
                </code>
              </div>
            ) : (
              <div className="rounded-md border border-dashed bg-muted/30 p-3 text-sm text-muted-foreground">
                Defina <code className="font-mono text-xs">MCP_PUBLIC_URL</code> no painel para exibir a URL pública.
                Exemplo: <code className="font-mono text-xs">https://mcp.seudominio.com</code>.
              </div>
            )}

            <div className="space-y-1.5">
              <p className="text-xs font-medium text-muted-foreground">1. Suba o servidor com transporte HTTP</p>
              <pre className="overflow-x-auto rounded-md border bg-muted/50 p-3 font-mono text-[11px] leading-relaxed">
                {SERVER_ENV}
              </pre>
            </div>

            <div className="space-y-1.5">
              <p className="text-xs font-medium text-muted-foreground">2. Crie o token do agente (arquivo com chmod 600)</p>
              <pre className="overflow-x-auto rounded-md border bg-muted/50 p-3 font-mono text-[11px] leading-relaxed">
                {TOKENS_JSON}
              </pre>
              <p className="text-xs text-muted-foreground">
                <code className="font-mono">read</code> vê apenas leitura;{" "}
                <code className="font-mono">write</code> adiciona drafts, envio e ack. O tenant e a key do bridge vêm do
                token, nunca do modelo.
              </p>
            </div>

            <div className="space-y-1.5">
              <p className="text-xs font-medium text-muted-foreground">3. Conecte o cliente</p>
              <pre className="overflow-x-auto rounded-md border bg-muted/50 p-3 font-mono text-[11px] leading-relaxed">
                {claudeCode}
              </pre>
              <p className="text-xs text-muted-foreground">
                Cursor ou outro cliente com suporte a MCP HTTP usam a mesma URL e o header:
              </p>
              <pre className="overflow-x-auto rounded-md border bg-muted/50 p-3 font-mono text-[11px] leading-relaxed">
                {CURSOR_JSON.replaceAll("<ENDPOINT>", endpointLabel)}
              </pre>
            </div>
          </CardContent>
        </Card>

        <div className="space-y-4">
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-sm">
                <ShieldCheck className="size-4 text-primary" aria-hidden />
                Segurança
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-3 text-sm text-muted-foreground">
              <p>Tokens são de uso único por agente/tenant; o arquivo precisa ser <code className="font-mono">600</code>.</p>
              <p>Rotação/revogação: edite o arquivo e reinicie o processo do MCP.</p>
              <p>
                O modo <code className="font-mono">read</code> esconde os tools de escrita e rejeita qualquer{" "}
                <code className="font-mono">tools/call</code> de escrita.
              </p>
              <p>Coloque TLS na frente; o processo deve escutar em loopback ou rede privada.</p>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-sm">
                <Terminal className="size-4 text-primary" aria-hidden />
                Teste rápido
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-2 text-sm text-muted-foreground">
              <p>Sem token, o endpoint responde <code className="font-mono">401</code>:</p>
              <pre className="overflow-x-auto rounded-md border bg-muted/50 p-3 font-mono text-[11px]">
                {`curl -i ${endpointLabel}`}
              </pre>
              <p>Com token, o cliente faz o handshake MCP e lista apenas os tools do escopo.</p>
            </CardContent>
          </Card>
        </div>
      </div>
    </>
  );
}
