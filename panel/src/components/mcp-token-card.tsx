"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { Check, Copy, KeyRound } from "lucide-react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

type CreatedToken = {
  id: string;
  name: string;
  scopes: string[];
  token: string;
  createdAt: string;
};

function copy(text: string, label: string) {
  navigator.clipboard
    .writeText(text)
    .then(() => toast.success(`${label} copiado.`))
    .catch(() => toast.error("Não foi possível copiar."));
}

export function McpTokenCard({ endpoint }: { endpoint: string | null }) {
  const router = useRouter();
  const [name, setName] = useState("Meu agente");
  const [write, setWrite] = useState(false);
  const [creating, setCreating] = useState(false);
  const [created, setCreated] = useState<CreatedToken | null>(null);

  const config = created
    ? JSON.stringify(
        {
          mcpServers: {
            whatsapp: {
              url: endpoint,
              headers: { Authorization: `Bearer ${created.token}` },
            },
          },
        },
        null,
        2,
      )
    : "";
  const claudeCommand = created
    ? `claude mcp add --transport http whatsapp \\\n  ${endpoint} \\\n  --header "Authorization: Bearer ${created.token}"`
    : "";

  async function create() {
    if (!endpoint) {
      toast.error("Endpoint ainda não configurado.");
      return;
    }
    setCreating(true);
    const response = await fetch("/api/panel/mcp-tokens", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ name, scopes: write ? ["read", "write"] : ["read"] }),
    });
    setCreating(false);
    if (!response.ok) {
      const body = (await response.json().catch(() => null)) as { error?: string } | null;
      toast.error(body?.error ?? "Falha ao criar token.");
      return;
    }
    setCreated((await response.json()) as CreatedToken);
    toast.success("Token criado. Copie agora: ele não será exibido novamente.");
    router.refresh();
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2 text-sm">
          <KeyRound className="size-4 text-primary" aria-hidden />
          Gerar token do agente
        </CardTitle>
        <CardDescription>O valor cru aparece uma vez. O painel guarda apenas o hash.</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="grid gap-3 sm:grid-cols-[1fr_auto] sm:items-end">
          <div className="space-y-1.5">
            <Label htmlFor="mcp-token-name">Nome do agente</Label>
            <Input
              id="mcp-token-name"
              value={name}
              onChange={(event) => setName(event.target.value)}
              placeholder="Claude Desktop"
              maxLength={80}
            />
          </div>
          <label className="flex items-center gap-2 pb-2 text-sm text-muted-foreground">
            <input
              type="checkbox"
              checked={write}
              onChange={(event) => setWrite(event.target.checked)}
              className="size-4 accent-primary"
            />
            permitir escrita
          </label>
        </div>

        <Button type="button" size="sm" onClick={create} disabled={creating || !endpoint}>
          {creating ? "Gerando…" : "Gerar token"}
        </Button>

        {!endpoint && <p className="text-xs text-muted-foreground">O endpoint precisa estar configurado no servidor.</p>}

        {created ? (
          <div className="space-y-3 rounded-md border border-highlight/40 bg-highlight/10 p-3">
            <div className="space-y-1">
              <p className="text-xs font-medium">Token de {created.name}</p>
              <code className="block break-all font-mono text-[11px]">{created.token}</code>
              <Button type="button" variant="outline" size="sm" onClick={() => copy(created.token, "Token")}>
                <Copy aria-hidden />
                Copiar token
              </Button>
            </div>
            <div className="space-y-1">
              <p className="text-xs font-medium">Configuração pronta para colar</p>
              <pre className="overflow-x-auto rounded-md border bg-background p-3 font-mono text-[11px]">{config}</pre>
              <Button type="button" variant="outline" size="sm" onClick={() => copy(config, "Configuração")}>
                <Copy aria-hidden />
                Copiar configuração
              </Button>
            </div>
            <div className="space-y-1">
              <p className="text-xs font-medium">Claude Code</p>
              <pre className="overflow-x-auto rounded-md border bg-background p-3 font-mono text-[11px]">{claudeCommand}</pre>
              <Button type="button" variant="outline" size="sm" onClick={() => copy(claudeCommand, "Comando")}>
                <Check aria-hidden />
                Copiar comando
              </Button>
            </div>
          </div>
        ) : null}
      </CardContent>
    </Card>
  );
}
