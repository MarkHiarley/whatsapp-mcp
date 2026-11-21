"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";
import { Ban } from "lucide-react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import type { Schema } from "@/lib/api";
import { bridgeDelete } from "@/lib/client-api";

export function KeyActions({ tenantId, tenantKey }: { tenantId: string; tenantKey: Schema["TenantKeyView"] }) {
  const router = useRouter();
  const [open, setOpen] = useState(false);
  const [pending, setPending] = useState(false);

  if (tenantKey.revoked_at) {
    return <span className="text-xs text-muted-foreground">—</span>;
  }

  async function revoke() {
    setPending(true);
    const result = await bridgeDelete(`/tenants/${tenantId}/keys/${tenantKey.id}`);
    setPending(false);
    if (!result.ok) {
      toast.error(result.error);
      return;
    }
    setOpen(false);
    toast.success("Key revogada.");
    router.refresh();
  }

  return (
    <>
      <Button type="button" variant="ghost" size="sm" onClick={() => setOpen(true)}>
        <Ban aria-hidden />
        Revogar
      </Button>
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Revogar key?</DialogTitle>
            <DialogDescription>
              A key <span className="font-medium">{tenantKey.label}</span> deixa de autenticar imediatamente. Esta ação
              não pode ser desfeita.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter showCloseButton>
            <Button variant="destructive" size="sm" onClick={revoke} disabled={pending}>
              <Ban aria-hidden />
              {pending ? "Revogando…" : "Revogar"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
