import { connection } from "next/server";

import { EmptyState } from "@/components/empty-state";
import { ErrorState } from "@/components/error-state";
import { PageHeader } from "@/components/page-header";
import { NewRuleDialog } from "@/components/rules/new-rule-dialog";
import { RuleActions } from "@/components/rules/rule-actions";
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
import { loadRules } from "@/lib/api";
import { formatDateTime } from "@/lib/format";
import { summarizeMatch } from "@/lib/rule-match";
import { requireTenantContext } from "@/lib/session";

export const instant = false;

export default async function RulesPage() {
  await connection();

  const context = await requireTenantContext();
  const { tenantId } = context;
  const canManage = context.isAdmin || context.organizationRole === "owner";
  const result = await loadRules();

  return (
    <>
      <PageHeader
        title="Regras"
        description="Regras definem quais mensagens viram eventos. O match usa AND entre campos e OR dentro das listas."
        actions={canManage ? <NewRuleDialog tenantId={tenantId} /> : undefined}
      />

      {!result.ok ? <ErrorState message={result.error} code={result.code} /> : null}

      {result.ok && result.data.length === 0 ? (
        <EmptyState
          title="Nenhuma regra"
          description="Crie a primeira regra para capturar mensagens por chat, remetente, texto ou flags."
          action={canManage ? <NewRuleDialog tenantId={tenantId} /> : undefined}
        />
      ) : null}

      {result.ok && result.data.length > 0 ? (
        <Card>
          <CardHeader className="border-b">
            <CardTitle className="text-sm">{result.data.length} regra(s)</CardTitle>
            <CardDescription>Desabilitar uma regra interrompe a captura, mas mantém o histórico.</CardDescription>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>id</TableHead>
                  <TableHead>nome</TableHead>
                  <TableHead>enabled</TableHead>
                  <TableHead>match</TableHead>
                  <TableHead>criada em</TableHead>
                  <TableHead className="text-right">ações</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {result.data.map((rule) => (
                  <TableRow key={rule.id}>
                    <TableCell className="max-w-32">
                      <span className="block truncate font-mono text-xs" title={rule.id}>
                        {rule.id}
                      </span>
                    </TableCell>
                    <TableCell className="max-w-40">
                      <span className="block truncate text-sm font-medium" title={rule.name}>
                        {rule.name}
                      </span>
                    </TableCell>
                    <TableCell>
                      <Badge variant={rule.enabled ? "default" : "outline"}>
                        {rule.enabled ? "habilitada" : "desabilitada"}
                      </Badge>
                    </TableCell>
                    <TableCell className="max-w-80 whitespace-normal">
                      <span className="line-clamp-2 text-xs text-muted-foreground">{summarizeMatch(rule.match)}</span>
                    </TableCell>
                    <TableCell className="text-xs whitespace-nowrap">{formatDateTime(rule.created_at)}</TableCell>
                    <TableCell>
                      {canManage ? (
                        <RuleActions tenantId={tenantId} rule={rule} />
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
