import { connection } from "next/server";
import Link from "next/link";

import { EmptyState } from "@/components/empty-state";
import { ErrorState } from "@/components/error-state";
import { PageHeader } from "@/components/page-header";
import { NewWebhookDialog } from "@/components/webhooks/new-webhook-dialog";
import { WebhookActions } from "@/components/webhooks/webhook-actions";
import { Badge } from "@/components/ui/badge";
import { buttonVariants } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { loadRules, loadWebhooks } from "@/lib/api";
import { formatDateTime } from "@/lib/format";
import { requireTenantContext } from "@/lib/session";

export const instant = false;

export default async function WebhooksPage() {
  await connection();

  const context = await requireTenantContext();
  const { tenantId } = context;
  const canManage = context.isAdmin || context.organizationRole === "owner";
  const [webhooksResult, rulesResult] = await Promise.all([loadWebhooks(), loadRules()]);
  const rules = rulesResult.ok ? rulesResult.data.map((rule) => ({ id: rule.id, name: rule.name })) : [];
  const ruleNames = new Map(rules.map((rule) => [rule.id, rule.name]));

  return (
    <>
      <PageHeader
        title="Webhooks"
        description="Cada webhook escuta uma regra e recebe POSTs com assinatura HMAC."
        actions={canManage ? <NewWebhookDialog tenantId={tenantId} rules={rules} /> : undefined}
      />

      {!rulesResult.ok ? (
        <p className="mb-3 text-xs text-muted-foreground">
          Regras indisponíveis: {rulesResult.error}
        </p>
      ) : null}

      {!webhooksResult.ok ? <ErrorState message={webhooksResult.error} code={webhooksResult.code} /> : null}

      {webhooksResult.ok && webhooksResult.data.length === 0 ? (
        <EmptyState
          title="Nenhum webhook"
          description="Configure um endpoint para receber os eventos das regras por HTTP."
          action={
            rules.length > 0 && canManage ? (
              <NewWebhookDialog tenantId={tenantId} rules={rules} />
            ) : (
              <Link href="/rules" className={buttonVariants({ variant: "outline", size: "sm" })}>
                Criar uma regra primeiro
              </Link>
            )
          }
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
                  <TableHead>regra</TableHead>
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
                    <TableCell className="max-w-40">
                      <span className="block truncate text-xs">
                        {ruleNames.get(webhook.rule_id) ?? webhook.rule_id}
                      </span>
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
