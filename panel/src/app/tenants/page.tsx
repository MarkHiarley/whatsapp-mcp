import { connection } from "next/server";

import { EmptyState } from "@/components/empty-state";
import { AdminNotConfigured, NotConfigured } from "@/components/not-configured";
import { PageHeader } from "@/components/page-header";
import { StatusBadge } from "@/components/status-badge";
import { NewTenantDialog } from "@/components/tenants/new-tenant-dialog";
import { TenantActions } from "@/components/tenants/tenant-actions";
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
import { loadTenants } from "@/lib/api";
import { TENANT_ID, bridgeConfigured } from "@/lib/bridge";
import { formatDateTime } from "@/lib/format";

export const instant = false;

export default async function TenantsPage() {
  await connection();

  if (!bridgeConfigured()) {
    return (
      <>
        <PageHeader title="Tenants" description="Administração de tenants do bridge." />
        <NotConfigured />
      </>
    );
  }

  const result = await loadTenants();
  const adminBlocked =
    !result.ok && (result.code === "PANEL_ADMIN_NOT_CONFIGURED" || result.code === "ADMIN_DISABLED" || result.code === "FORBIDDEN");

  return (
    <>
      <PageHeader
        title="Tenants"
        description="Visão administrativa. Provisionar um tenant devolve o token inicial uma única vez."
        actions={adminBlocked ? undefined : <NewTenantDialog />}
      />

      {adminBlocked ? <AdminNotConfigured message={result.error} /> : null}

      {!result.ok && !adminBlocked ? (
        <Card className="border-destructive/30">
          <CardHeader>
            <CardTitle className="text-destructive">Não foi possível listar os tenants</CardTitle>
            <CardDescription>{result.error}</CardDescription>
          </CardHeader>
          {result.code ? (
            <CardContent>
              <code className="rounded bg-muted px-1.5 py-0.5 font-mono text-xs">{result.code}</code>
            </CardContent>
          ) : null}
        </Card>
      ) : null}

      {result.ok && result.data.length === 0 ? (
        <EmptyState
          title="Nenhum tenant"
          description="Provisione o primeiro tenant para começar."
          action={<NewTenantDialog />}
        />
      ) : null}

      {result.ok && result.data.length > 0 ? (
        <Card>
          <CardHeader className="border-b">
            <CardTitle className="text-sm">{result.data.length} tenant(s)</CardTitle>
            <CardDescription>
              Pareamento de outros tenants ainda não é feito por este painel; o QR abaixo é do tenant configurado.
            </CardDescription>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>id</TableHead>
                  <TableHead>label</TableHead>
                  <TableHead>status</TableHead>
                  <TableHead>keys</TableHead>
                  <TableHead>criado em</TableHead>
                  <TableHead className="text-right">ações</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {result.data.map((tenant) => (
                  <TableRow key={tenant.id}>
                    <TableCell className="max-w-40">
                      <span className="block truncate font-mono text-xs" title={tenant.id}>
                        {tenant.id}
                      </span>
                    </TableCell>
                    <TableCell className="max-w-48">
                      <span className="block truncate text-sm" title={tenant.label}>
                        {tenant.label}
                      </span>
                    </TableCell>
                    <TableCell>
                      <StatusBadge status={tenant.status} />
                    </TableCell>
                    <TableCell>
                      <Badge variant="secondary" className="tabular-nums">
                        {tenant.keys}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-xs whitespace-nowrap">{formatDateTime(tenant.created_at)}</TableCell>
                    <TableCell>
                      <div className="flex items-center justify-end gap-2">
                        {tenant.id === TENANT_ID ? (
                          <Badge variant="outline" className="hidden sm:inline-flex">
                            atual
                          </Badge>
                        ) : null}
                        <TenantActions tenant={tenant} currentTenantId={TENANT_ID} />
                      </div>
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
