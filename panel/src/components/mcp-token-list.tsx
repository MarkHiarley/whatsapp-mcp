"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { Ban } from "lucide-react";
import { toast } from "sonner";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";

export type McpTokenRow = {
  id: string;
  name: string;
  scopes: string;
  tenantId: string;
  createdAt: string;
  lastUsedAt: string | null;
  revokedAt: string | null;
};

function scopesOf(value: string): string[] {
  try {
    const parsed = JSON.parse(value) as unknown;
    return Array.isArray(parsed) ? parsed.filter((item): item is string => typeof item === "string") : ["read"];
  } catch {
    return ["read"];
  }
}

function when(value: string | null): string {
  return value ? new Date(value).toLocaleString("pt-BR") : "nunca";
}

export function McpTokenList({ tokens }: { tokens: McpTokenRow[] }) {
  const router = useRouter();
  const [revoking, setRevoking] = useState<string | null>(null);

  async function revoke(id: string, name: string) {
    if (!window.confirm(`Revogar o token “${name}”? O agente perde acesso imediatamente.`)) {
      return;
    }
    setRevoking(id);
    const response = await fetch(`/api/panel/mcp-tokens/${id}`, { method: "DELETE" });
    setRevoking(null);
    if (!response.ok) {
      toast.error("Falha ao revogar o token.");
      return;
    }
    toast.success("Token revogado.");
    router.refresh();
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-sm">Tokens emitidos</CardTitle>
        <CardDescription>O valor cru não é exibido novamente. Revogar invalida o agente na próxima chamada.</CardDescription>
      </CardHeader>
      <CardContent>
        {tokens.length === 0 ? (
          <p className="text-sm text-muted-foreground">Nenhum token emitido para este tenant.</p>
        ) : (
          <div className="divide-y rounded-md border">
            {tokens.map((token) => {
              const revoked = Boolean(token.revokedAt);
              return (
                <div key={token.id} className="flex flex-wrap items-center gap-3 px-3 py-3">
                  <div className="min-w-0 flex-1 space-y-1">
                    <div className="flex flex-wrap items-center gap-2">
                      <p className="truncate text-sm font-medium">{token.name}</p>
                      {revoked ? <Badge variant="outline">revogado</Badge> : <Badge>ativo</Badge>}
                      {scopesOf(token.scopes).map((scope) => (
                        <Badge key={scope} variant="outline">
                          {scope}
                        </Badge>
                      ))}
                    </div>
                    <p className="font-mono text-[10px] text-muted-foreground">{token.tenantId}</p>
                    <p className="text-[11px] text-muted-foreground">
                      criado {when(token.createdAt)} · último uso {when(token.lastUsedAt)}
                    </p>
                  </div>
                  {revoked ? null : (
                    <Button
                      type="button"
                      variant="destructive"
                      size="sm"
                      disabled={revoking === token.id}
                      onClick={() => revoke(token.id, token.name)}
                    >
                      <Ban aria-hidden />
                      {revoking === token.id ? "Revogando…" : "Revogar"}
                    </Button>
                  )}
                </div>
              );
            })}
          </div>
        )}
      </CardContent>
    </Card>
  );
}
