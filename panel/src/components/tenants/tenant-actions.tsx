"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";
import { Trash2 } from "lucide-react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import type { Schema } from "@/lib/api";
import { bridgeDelete } from "@/lib/client-api";

export function TenantActions({ tenant, currentTenantId }: { tenant: Schema["TenantView"]; currentTenantId: string }) {
  const router = useRouter();
  const [open, setOpen] = useState(false);
  const [pending, setPending] = useState(false);
  const isCurrent = tenant.id === currentTenantId;

  async function remove() {
    setPending(true);
    const result = await bridgeDelete(`/tenants/${tenant.id}`);
    setPending(false);
    if (!result.ok) {
      toast.error(result.error);
      return;
    }
    setOpen(false);
    toast.success(`Tenant ${tenant.id} removido.`);
    router.refresh();
  }

  return (
    <>
      <Button
        type="button"
        variant="ghost"
        size="sm"
        disabled={isCurrent}
        title={isCurrent ? "Este é o tenant configurado no painel" : "Remover tenant"}
        onClick={() => setOpen(true)}
      >
        <Trash2 aria-hidden />
        Remover
      </Button>
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Remover tenant?</DialogTitle>
            <DialogDescription>
              O tenant <span className="font-mono text-xs">{tenant.id}</span> é desconectado e seus arquivos
              (sessão, mensagens e mídia) são removidos. Esta ação não pode ser desfeita.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter showCloseButton>
            <Button variant="destructive" size="sm" onClick={remove} disabled={pending}>
              <Trash2 aria-hidden />
              {pending ? "Removendo…" : "Remover tenant"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
