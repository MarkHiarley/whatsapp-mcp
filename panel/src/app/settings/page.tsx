import { connection } from "next/server";
import Link from "next/link";
import { ExternalLink, ShieldCheck } from "lucide-react";

import { NotConfigured } from "@/components/not-configured";
import { PageHeader } from "@/components/page-header";
import { QrPairing } from "@/components/pairing/qr-pairing";
import { StatusBadge } from "@/components/status-badge";
import { buttonVariants } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { loadStatus } from "@/lib/api";
import { BRIDGE_URL, TENANT_ID, bridgeConfigured } from "@/lib/bridge";
import { formatDateTime } from "@/lib/format";

export const instant = false;

export default async function SettingsPage() {
  await connection();

  if (!bridgeConfigured()) {
    return (
      <>
        <PageHeader title="Pareamento" description="QR e estado da sessão do tenant configurado." />
        <NotConfigured />
      </>
    );
  }

  const statusResult = await loadStatus();

  return (
    <>
      <PageHeader
        title="Pareamento / QR"
        description="Vincule o número do tenant configurado escaneando o QR com o WhatsApp."
      />

      <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_320px]">
        <Card>
          <CardHeader className="border-b">
            <CardTitle className="text-sm">Sessão do tenant</CardTitle>
            <CardDescription>
              O stream usa SSE pelo BFF: o token fica no servidor e não é exposto ao navegador.
            </CardDescription>
          </CardHeader>
          <CardContent>
            <QrPairing tenantId={TENANT_ID} initialStatus={statusResult.ok ? statusResult.data.status : "disconnected"} />
          </CardContent>
        </Card>

        <div className="space-y-4">
          <Card size="sm">
            <CardHeader>
              <CardTitle className="text-sm">Tenant atual</CardTitle>
            </CardHeader>
            <CardContent className="space-y-2 text-sm">
              <p className="font-mono text-xs">{TENANT_ID}</p>
              {statusResult.ok ? (
                <div className="flex flex-wrap items-center gap-2">
                  <StatusBadge status={statusResult.data.status} />
                  {statusResult.data.jid ? (
                    <span className="font-mono text-xs text-muted-foreground">{statusResult.data.jid}</span>
                  ) : null}
                </div>
              ) : (
                <p className="text-xs text-destructive">{statusResult.error}</p>
              )}
              {statusResult.ok ? (
                <p className="text-xs text-muted-foreground">
                  Criado em {formatDateTime(statusResult.data.created_at)} · {statusResult.data.keys} key(s)
                </p>
              ) : null}
            </CardContent>
          </Card>

          <Card size="sm">
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-sm">
                <ShieldCheck className="size-4" aria-hidden />
                Bridge
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-2 text-xs text-muted-foreground">
              <Link
                href={BRIDGE_URL}
                target="_blank"
                rel="noreferrer"
                className="inline-flex items-center gap-1 font-mono break-all hover:text-foreground"
              >
                <ExternalLink className="size-3.5 shrink-0" aria-hidden />
                {BRIDGE_URL}
              </Link>
              <p>
                Trocar o tenant exige atualizar <code className="font-mono">BRIDGE_TENANT_ID</code> e{" "}
                <code className="font-mono">BRIDGE_API_TOKEN</code> no servidor do painel e reiniciar o processo.
              </p>
              <Link href="/tenants" className={buttonVariants({ variant: "outline", size: "sm" })}>
                Administrar tenants
              </Link>
            </CardContent>
          </Card>
        </div>
      </div>
    </>
  );
}
