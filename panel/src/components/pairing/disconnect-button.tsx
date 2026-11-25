"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";
import { LogOut } from "lucide-react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import type { TenantStatusValue } from "@/lib/status";
import { bridgePost } from "@/lib/client-api";

export function DisconnectButton({
  tenantId,
  status,
  canManage,
}: {
  tenantId: string;
  status: TenantStatusValue;
  canManage: boolean;
}) {
  const router = useRouter();
  const [pending, setPending] = useState(false);
  if (!canManage || status === "disconnected" || status === "logged_out") {
    return null;
  }

  async function disconnect() {
    if (!window.confirm("Desconectar este WhatsApp? A sessão neste dispositivo será encerrada e será preciso escanear um novo QR.")) {
      return;
    }
    setPending(true);
    const result = await bridgePost(`/tenants/${tenantId}/logout`);
    setPending(false);
    if (!result.ok) {
      toast.error(result.error);
      return;
    }
    toast.success("WhatsApp desconectado. Escaneie o QR para parear outra conta.");
    router.refresh();
  }

  return (
    <Button type="button" variant="outline" size="sm" onClick={disconnect} disabled={pending}>
      <LogOut aria-hidden />
      {pending ? "Desconectando…" : "Desconectar WhatsApp"}
    </Button>
  );
}
