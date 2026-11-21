import { connection } from "next/server";

import { EmptyState } from "@/components/empty-state";
import { ErrorState } from "@/components/error-state";
import { KeyActions } from "@/components/keys/key-actions";
import { NewKeyDialog } from "@/components/keys/new-key-dialog";
import { NotConfigured } from "@/components/not-configured";
import { PageHeader } from "@/components/page-header";
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
import { loadKeys } from "@/lib/api";
import { TENANT_ID, bridgeConfigured } from "@/lib/bridge";
import { formatDateTime } from "@/lib/format";

export const instant = false;

export default async function KeysPage() {
  await connection();

  if (!bridgeConfigured()) {
    return (
      <>
        <PageHeader title="Chaves" description="Credenciais do tenant configurado." />
        <NotConfigured />
      </>
    );
  }

  const result = await loadKeys();

  return (
    <>
      <PageHeader
        title="Chaves"
        description="Cada key autentica uma integração. Tokens nunca são exibidos depois da criação."
        actions={<NewKeyDialog tenantId={TENANT_ID} />}
      />

      {!result.ok ? <ErrorState message={result.error} code={result.code} /> : null}

      {result.ok && result.data.length === 0 ? (
        <EmptyState
          title="Nenhuma key"
          description="Emita uma key para conectar integrações a este tenant."
          action={<NewKeyDialog tenantId={TENANT_ID} />}
        />
      ) : null}

      {result.ok && result.data.length > 0 ? (
        <Card>
          <CardHeader className="border-b">
            <CardTitle className="text-sm">{result.data.length} key(s)</CardTitle>
            <CardDescription>Revogar interrompe o acesso imediatamente.</CardDescription>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>label</TableHead>
                  <TableHead>id</TableHead>
                  <TableHead>criada em</TableHead>
                  <TableHead>last_used_at</TableHead>
                  <TableHead>status</TableHead>
                  <TableHead className="text-right">ações</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {result.data.map((tenantKey) => (
                  <TableRow key={tenantKey.id}>
                    <TableCell className="max-w-40">
                      <span className="block truncate text-sm font-medium" title={tenantKey.label}>
                        {tenantKey.label}
                      </span>
                    </TableCell>
                    <TableCell className="max-w-32">
                      <span className="block truncate font-mono text-xs" title={tenantKey.id}>
                        {tenantKey.id}
                      </span>
                    </TableCell>
                    <TableCell className="text-xs whitespace-nowrap">{formatDateTime(tenantKey.created_at)}</TableCell>
                    <TableCell className="text-xs whitespace-nowrap">{formatDateTime(tenantKey.last_used_at)}</TableCell>
                    <TableCell>
                      {tenantKey.revoked_at ? (
                        <Badge variant="destructive" title={formatDateTime(tenantKey.revoked_at)}>
                          revogada
                        </Badge>
                      ) : (
                        <Badge variant="default">ativa</Badge>
                      )}
                    </TableCell>
                    <TableCell>
                      <KeyActions tenantId={TENANT_ID} tenantKey={tenantKey} />
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
