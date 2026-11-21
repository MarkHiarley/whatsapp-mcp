"use client";

import { useRouter } from "next/navigation";
import { useState, type FormEvent } from "react";
import { Plus } from "lucide-react";
import { toast } from "sonner";

import { SecretDialog } from "@/components/secret-dialog";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import type { Schema } from "@/lib/api";
import { bridgePost } from "@/lib/client-api";

export function NewKeyDialog({ tenantId }: { tenantId: string }) {
  const router = useRouter();
  const [open, setOpen] = useState(false);
  const [saving, setSaving] = useState(false);
  const [label, setLabel] = useState("");
  const [token, setToken] = useState<string | null>(null);
  const [tokenOpen, setTokenOpen] = useState(false);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!label.trim()) {
      toast.error("Informe um label para a key.");
      return;
    }

    setSaving(true);
    const result = await bridgePost<{ key: Schema["TenantKeyView"]; token: string }>(`/tenants/${tenantId}/keys`, {
      label: label.trim(),
    });
    setSaving(false);

    if (!result.ok) {
      toast.error(result.error);
      return;
    }

    setLabel("");
    setOpen(false);
    setToken(result.data?.token ?? null);
    setTokenOpen(true);
    toast.success("Key criada.");
    router.refresh();
  }

  return (
    <>
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogTrigger render={<Button size="sm" />}>
          <Plus aria-hidden />
          Nova key
        </DialogTrigger>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Nova key</DialogTitle>
            <DialogDescription>
              Keys autenticam integrações deste tenant. O token só é exibido uma vez, após a criação.
            </DialogDescription>
          </DialogHeader>
          <form onSubmit={submit} className="space-y-3">
            <div className="space-y-1.5">
              <Label htmlFor="key-label">Label</Label>
              <Input
                id="key-label"
                value={label}
                onChange={(event) => setLabel(event.target.value)}
                placeholder="n8n produção"
                required
              />
            </div>
            <DialogFooter showCloseButton>
              <Button type="submit" size="sm" disabled={saving}>
                {saving ? "Criando…" : "Criar key"}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      <SecretDialog
        open={tokenOpen}
        onOpenChange={setTokenOpen}
        title="Token da key"
        description="Copie o token agora. Ele não pode ser recuperado depois."
        secret={token}
      />
    </>
  );
}
