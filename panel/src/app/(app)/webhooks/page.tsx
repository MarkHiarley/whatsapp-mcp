import { connection } from "next/server";

import { EmptyState } from "@/components/empty-state";
import { ErrorState } from "@/components/error-state";
import { PageHeader } from "@/components/page-header";
import { NewWebhookDialog } from "@/components/webhooks/new-webhook-dialog";
import { WebhookActions } from "@/components/webhooks/webhook-actions";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { loadWebhooks } from "@/lib/api";
import { formatDateTime } from "@/lib/format";
import { requireTenantContext } from "@/lib/session";

export const instant = false;

export default async function WebhooksPage() {
  await connection();

  const context = await requireTenantContext();
  const { tenantId } = context;
  const canManage = context.isAdmin || context.organizationRole === "owner";
  const webhooksResult = await loadWebhooks();

  return (
    <>
      <PageHeader
        title="Webhooks"
        description="Cada webhook recebe POSTs assinados com HMAC dos eventos que passam nos filtros escolhidos."
        actions={canManage ? <NewWebhookDialog tenantId={tenantId} /> : undefined}
      />

      {!webhooksResult.ok ? <ErrorState message={webhooksResult.error} code={webhooksResult.code} /> : null}

      {webhooksResult.ok && webhooksResult.data.length === 0 ? (
        <EmptyState
          title="Nenhum webhook"
          description="Configure um endpoint para receber os eventos por HTTP."
          action={canManage ? <NewWebhookDialog tenantId={tenantId} /> : undefined}
        />
      ) : null}

      {webhooksResult.ok && webhooksResult.data.length > 0 ? (
        <Card>
          <CardHeader className="border-b">
            <CardTitle className="text-sm">{webhooksResult.data.length} webhook(s)</CardTitle>
            <CardDescription>Secrets são exibidos apenas na criação e na rotação.</CardDescription>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>URL</TableHead>
                  <TableHead>autenticação</TableHead>
                  <TableHead>enabled</TableHead>
                  <TableHead>criado em</TableHead>
                  <TableHead className="text-right">ações</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {webhooksResult.data.map((webhook) => (
                  <TableRow key={webhook.id}>
                    <TableCell className="max-w-72">
                      <span className="block truncate font-mono text-xs" title={webhook.url}>
                        {webhook.url}
                      </span>
                    </TableCell>
                    <TableCell className="max-w-44">
                      {webhook.header_names && webhook.header_names.length > 0 ? (
                        <div className="flex flex-wrap gap-1">
                          {webhook.header_names.map((name) => (
                            <Badge key={name} variant="outline">
                              {name}
                            </Badge>
                          ))}
                        </div>
                      ) : (
                        <span className="text-xs text-muted-foreground">HMAC</span>
                      )}
                    </TableCell>
                    <TableCell>
                      <Badge variant={webhook.enabled ? "default" : "outline"}>
                        {webhook.enabled ? "ativo" : "inativo"}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-xs whitespace-nowrap">{formatDateTime(webhook.created_at)}</TableCell>
                    <TableCell>
                      {canManage ? (
                        <WebhookActions tenantId={tenantId} webhook={webhook} />
                      ) : (
                        <Badge variant="outline">somente leitura</Badge>
                      )}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      ) : null}
    </>
  );
}
