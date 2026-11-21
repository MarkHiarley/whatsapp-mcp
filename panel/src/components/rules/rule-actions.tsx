"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";
import { Power, PowerOff, Trash2 } from "lucide-react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import type { Schema } from "@/lib/api";
import { bridgeDelete, bridgePatch } from "@/lib/client-api";

export function RuleActions({ tenantId, rule }: { tenantId: string; rule: Schema["EventRule"] }) {
  const router = useRouter();
  const [pending, setPending] = useState(false);
  const [confirmOpen, setConfirmOpen] = useState(false);

  async function toggle() {
    setPending(true);
    const result = await bridgePatch<{ rule: Schema["EventRule"] }>(`/tenants/${tenantId}/rules/${rule.id}`, {
      enabled: !rule.enabled,
    });
    setPending(false);
    if (!result.ok) {
      toast.error(result.error);
      return;
    }
    toast.success(result.data?.rule.enabled ? "Regra habilitada." : "Regra desabilitada.");
    router.refresh();
  }

  async function remove() {
    setPending(true);
    const result = await bridgeDelete(`/tenants/${tenantId}/rules/${rule.id}`);
    setPending(false);
    if (!result.ok) {
      toast.error(result.error);
      return;
    }
    setConfirmOpen(false);
    toast.success("Regra removida.");
    router.refresh();
  }

  return (
    <div className="flex items-center justify-end gap-1">
      <Button type="button" variant="ghost" size="sm" onClick={toggle} disabled={pending}>
        {rule.enabled ? <PowerOff aria-hidden /> : <Power aria-hidden />}
        <span className="hidden sm:inline">{rule.enabled ? "Desabilitar" : "Habilitar"}</span>
      </Button>
      <Button type="button" variant="ghost" size="icon-sm" onClick={() => setConfirmOpen(true)} disabled={pending}>
        <Trash2 aria-hidden />
        <span className="sr-only">Remover regra</span>
      </Button>

      <Dialog open={confirmOpen} onOpenChange={setConfirmOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Remover regra?</DialogTitle>
            <DialogDescription>
              A regra <span className="font-medium">{rule.name}</span> será removida. Webhooks e eventos já gravados
              não são afetados.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter showCloseButton>
            <Button variant="destructive" size="sm" onClick={remove} disabled={pending}>
              <Trash2 aria-hidden />
              {pending ? "Removendo…" : "Remover"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
