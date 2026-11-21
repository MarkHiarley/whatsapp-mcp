"use client";

import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { ListChecks, RotateCw, Trash2 } from "lucide-react";
import { toast } from "sonner";

import { SecretDialog } from "@/components/secret-dialog";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import type { Schema } from "@/lib/api";
import { bridgeDelete, bridgeGet, bridgePost } from "@/lib/client-api";
import { formatDateTime } from "@/lib/format";

export function WebhookActions({ tenantId, webhook }: { tenantId: string; webhook: Schema["WebhookView"] }) {
  const router = useRouter();
  const [pending, setPending] = useState(false);
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [secret, setSecret] = useState<string | null>(null);
  const [secretOpen, setSecretOpen] = useState(false);
  const [deliveriesOpen, setDeliveriesOpen] = useState(false);
  const [deliveries, setDeliveries] = useState<Schema["DeliveryView"][]>([]);
  const [loadingDeliveries, setLoadingDeliveries] = useState(false);
  const [deliveriesError, setDeliveriesError] = useState<string | null>(null);

  async function rotate() {
    setPending(true);
    const result = await bridgePost<{ secret: string }>(`/tenants/${tenantId}/webhooks/${webhook.id}/secret`);
    setPending(false);
    if (!result.ok) {
      toast.error(result.error);
      return;
    }
    setSecret(result.data?.secret ?? null);
    setSecretOpen(true);
    toast.success("Secret rotacionado. O valor antigo deixa de valer.");
  }

  async function remove() {
    setPending(true);
    const result = await bridgeDelete(`/tenants/${tenantId}/webhooks/${webhook.id}`);
    setPending(false);
    if (!result.ok) {
      toast.error(result.error);
      return;
    }
    setConfirmOpen(false);
    toast.success("Webhook removido.");
    router.refresh();
  }

  useEffect(() => {
    if (!deliveriesOpen) {
      return;
    }
    let active = true;
    bridgeGet<{ deliveries: Schema["DeliveryView"][] }>(
      `/tenants/${tenantId}/webhooks/${webhook.id}/deliveries?limit=25`,
    ).then((result) => {
      if (!active) {
        return;
      }
      setLoadingDeliveries(false);
      if (!result.ok) {
        setDeliveriesError(result.error);
        setDeliveries([]);
        return;
      }
      setDeliveries(result.data?.deliveries ?? []);
    });
    return () => {
      active = false;
    };
  }, [deliveriesOpen, tenantId, webhook.id]);

  return (
    <div className="flex items-center justify-end gap-1">
      <Button
        type="button"
        variant="ghost"
        size="sm"
        onClick={() => {
          setLoadingDeliveries(true);
          setDeliveriesError(null);
          setDeliveriesOpen(true);
        }}
      >
        <ListChecks aria-hidden />
        <span className="hidden md:inline">Entregas</span>
      </Button>
      <Button type="button" variant="ghost" size="sm" onClick={rotate} disabled={pending}>
        <RotateCw aria-hidden />
        <span className="hidden md:inline">Rotacionar</span>
      </Button>
      <Button type="button" variant="ghost" size="icon-sm" onClick={() => setConfirmOpen(true)} disabled={pending}>
        <Trash2 aria-hidden />
        <span className="sr-only">Remover webhook</span>
      </Button>

      <SecretDialog
        open={secretOpen}
        onOpenChange={setSecretOpen}
        title="Novo secret"
        description="Guarde o valor agora: ele não será mostrado novamente."
        secret={secret}
      />

      <Dialog open={deliveriesOpen} onOpenChange={setDeliveriesOpen}>
        <DialogContent className="max-h-[85vh] overflow-y-auto sm:max-w-3xl">
          <DialogHeader>
            <DialogTitle>Entregas do webhook</DialogTitle>
            <DialogDescription className="truncate font-mono text-xs">{webhook.url}</DialogDescription>
          </DialogHeader>
          {loadingDeliveries ? (
            <div className="space-y-2">
              <div className="h-8 animate-pulse rounded-md bg-muted" />
              <div className="h-8 animate-pulse rounded-md bg-muted" />
              <div className="h-8 animate-pulse rounded-md bg-muted" />
            </div>
          ) : deliveriesError ? (
            <p className="text-sm text-destructive">{deliveriesError}</p>
          ) : deliveries.length === 0 ? (
            <p className="text-sm text-muted-foreground">Nenhuma entrega registrada para este webhook.</p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>seq</TableHead>
                  <TableHead>attempts</TableHead>
                  <TableHead>status</TableHead>
                  <TableHead>erro</TableHead>
                  <TableHead>entregue em</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {deliveries.map((delivery) => (
                  <TableRow key={delivery.seq}>
                    <TableCell className="font-mono text-xs tabular-nums">{delivery.seq}</TableCell>
                    <TableCell className="text-xs tabular-nums">{delivery.attempts}</TableCell>
                    <TableCell className="text-xs">
                      {delivery.delivered_at ? (
                        <span className="font-medium text-emerald-600 dark:text-emerald-400">entregue</span>
                      ) : delivery.last_status ? (
                        <span className="text-destructive">HTTP {delivery.last_status}</span>
                      ) : (
                        <span className="text-muted-foreground">pendente</span>
                      )}
                    </TableCell>
                    <TableCell className="max-w-60 whitespace-normal text-xs text-muted-foreground">
                      <span className="line-clamp-2">{delivery.last_error ?? "—"}</span>
                    </TableCell>
                    <TableCell className="text-xs whitespace-nowrap">
                      {delivery.delivered_at ? formatDateTime(delivery.delivered_at) : formatDateTime(delivery.next_attempt_at)}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
          <DialogFooter showCloseButton />
        </DialogContent>
      </Dialog>

      <Dialog open={confirmOpen} onOpenChange={setConfirmOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Remover webhook?</DialogTitle>
            <DialogDescription>
              As entregas param imediatamente para <span className="font-mono text-xs break-all">{webhook.url}</span>.
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
