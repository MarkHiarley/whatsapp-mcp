"use client";

import { useRouter } from "next/navigation";
import { useState, type FormEvent } from "react";
import { Plus } from "lucide-react";
import { toast } from "sonner";

import { CopyButton } from "@/components/copy-button";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import type { Schema } from "@/lib/api";
import { bridgePost } from "@/lib/client-api";

const TENANT_ID_PATTERN = /^[a-z0-9][a-z0-9-]{1,31}$/;

export function NewTenantDialog() {
  const router = useRouter();
  const [open, setOpen] = useState(false);
  const [saving, setSaving] = useState(false);
  const [id, setId] = useState("");
  const [label, setLabel] = useState("");
  const [token, setToken] = useState<string | null>(null);
  const [createdId, setCreatedId] = useState<string | null>(null);
  const [tokenOpen, setTokenOpen] = useState(false);
  const [selecting, setSelecting] = useState(false);

  async function pairNow() {
    if (!createdId) {
      return;
    }
    setSelecting(true);
    const response = await fetch("/api/panel/tenant", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ tenant: createdId }),
    });
    setSelecting(false);
    if (!response.ok) {
      toast.error("Tenant criado, mas não foi possível selecioná-lo no painel.");
      return;
    }
    setTokenOpen(false);
    router.push("/settings");
    router.refresh();
  }

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const tenantId = id.trim().toLowerCase();
    if (!TENANT_ID_PATTERN.test(tenantId)) {
      toast.error("O id deve casar com ^[a-z0-9][a-z0-9-]{1,31}$.");
      return;
    }
    if (!label.trim()) {
      toast.error("Informe um label para o tenant.");
      return;
    }

    setSaving(true);
    const result = await bridgePost<{ tenant: Schema["TenantView"]; token: string }>("/tenants", {
      id: tenantId,
      label: label.trim(),
    });
    setSaving(false);

    if (!result.ok) {
      toast.error(result.error);
      return;
    }

    setCreatedId(result.data?.tenant.id ?? tenantId);
    setToken(result.data?.token ?? null);
    setId("");
    setLabel("");
    setOpen(false);
    setTokenOpen(true);
    toast.success("Tenant criado. Guarde o token do primeiro acesso.");
    router.refresh();
  }

  return (
    <>
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogTrigger render={<Button size="sm" />}>
          <Plus aria-hidden />
          Novo tenant
        </DialogTrigger>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Novo tenant</DialogTitle>
            <DialogDescription>
              Cria um tenant isolado e emite a primeira key. O token é exibido uma única vez.
            </DialogDescription>
          </DialogHeader>
          <form onSubmit={submit} className="space-y-3">
            <div className="space-y-1.5">
              <Label htmlFor="tenant-id">id</Label>
              <Input
                id="tenant-id"
                value={id}
                onChange={(event) => setId(event.target.value)}
                placeholder="cliente-x"
                pattern="[a-z0-9][a-z0-9-]{1,31}"
                className="font-mono"
                required
              />
              <p className="text-xs text-muted-foreground">Minúsculas, números e hífens (2 a 32 caracteres).</p>
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="tenant-label">Label</Label>
              <Input
                id="tenant-label"
                value={label}
                onChange={(event) => setLabel(event.target.value)}
                placeholder="Cliente X"
                required
              />
            </div>
            <DialogFooter showCloseButton>
              <Button type="submit" size="sm" disabled={saving}>
                {saving ? "Criando…" : "Criar tenant"}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      <Dialog open={tokenOpen} onOpenChange={setTokenOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>Token do tenant {createdId}</DialogTitle>
            <DialogDescription>
              Use este token como <code className="font-mono text-xs">BRIDGE_API_TOKEN</code> do painel desse tenant
              ou na integração. Ele não pode ser recuperado depois.
            </DialogDescription>
          </DialogHeader>
          {token ? (
            <pre className="max-h-40 overflow-auto rounded-lg bg-muted p-3 font-mono text-xs break-all whitespace-pre-wrap">
              {token}
            </pre>
          ) : (
            <p className="text-sm text-muted-foreground">Nenhum token retornado.</p>
          )}
          <DialogFooter showCloseButton>
            {token ? <CopyButton value={token} /> : null}
            <Button type="button" size="sm" onClick={pairNow} disabled={selecting || !createdId}>
              {selecting ? "Selecionando…" : "Parear agora"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
