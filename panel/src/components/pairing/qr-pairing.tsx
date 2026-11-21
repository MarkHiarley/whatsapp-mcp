"use client";

import { useRouter } from "next/navigation";
import { useEffect, useRef, useState } from "react";
import { QRCodeSVG } from "qrcode.react";
import { CircleCheck, RefreshCw, TriangleAlert } from "lucide-react";

import type { TenantStatusValue } from "@/lib/status";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";

type StreamState = "idle" | "live" | "closed" | "error";

const TERMINAL_STATUSES: TenantStatusValue[] = ["connected", "logged_out"];

function isTenantStatus(value: unknown): value is TenantStatusValue {
  return value === "disconnected" || value === "pairing" || value === "connected" || value === "logged_out";
}

export function QrPairing({ tenantId, initialStatus }: { tenantId: string; initialStatus: TenantStatusValue }) {
  const router = useRouter();
  const [code, setCode] = useState<string | null>(null);
  const [status, setStatus] = useState<TenantStatusValue>(initialStatus);
  const [stream, setStream] = useState<StreamState>("live");
  const [attempt, setAttempt] = useState(0);
  const doneRef = useRef(false);

  useEffect(() => {
    doneRef.current = false;
    if (TERMINAL_STATUSES.includes(initialStatus)) {
      return;
    }

    const refreshKey = `panel:qr-refreshed:${tenantId}`;
    const source = new EventSource(`/api/bridge/tenants/${encodeURIComponent(tenantId)}/qr`);

    function handleQr(event: MessageEvent<string>) {
      try {
        const payload = JSON.parse(event.data) as { code?: unknown };
        if (typeof payload.code === "string" && payload.code.length > 0) {
          setCode(payload.code);
        }
      } catch {
        // Ignore malformed SSE payloads and keep the stream alive.
      }
    }

    function handleStatus(event: MessageEvent<string>) {
      try {
        const payload = JSON.parse(event.data) as { status?: unknown };
        if (isTenantStatus(payload.status)) {
          setStatus(payload.status);
          if (TERMINAL_STATUSES.includes(payload.status)) {
            doneRef.current = true;
            source.close();
            setStream("closed");
            if (payload.status === "connected" && !sessionStorage.getItem(refreshKey)) {
              sessionStorage.setItem(refreshKey, "1");
              router.refresh();
            }
          }
        }
      } catch {
        // Ignore malformed SSE payloads.
      }
    }

    source.addEventListener("qr", handleQr as EventListener);
    source.addEventListener("status", handleStatus as EventListener);
    source.onerror = () => {
      if (!doneRef.current) {
        setStream("error");
      }
    };

    return () => {
      doneRef.current = true;
      source.removeEventListener("qr", handleQr as EventListener);
      source.removeEventListener("status", handleStatus as EventListener);
      source.close();
    };
  }, [tenantId, router, attempt, initialStatus]);

  const displayStatus = TERMINAL_STATUSES.includes(initialStatus) ? initialStatus : status;
  const displayStream: StreamState = TERMINAL_STATUSES.includes(displayStatus) ? "closed" : stream;
  const connected = displayStatus === "connected";

  return (
    <div className="flex flex-col items-center gap-4 py-2">
      {connected ? (
        <div className="flex flex-col items-center gap-2 py-8 text-center">
          <CircleCheck className="size-10 text-emerald-500" aria-hidden />
          <p className="font-medium">Sessão conectada</p>
          <p className="text-sm text-muted-foreground">O stream de pareamento foi encerrado pelo bridge.</p>
        </div>
      ) : code ? (
        <div className="rounded-xl bg-white p-4 shadow-sm ring-1 ring-black/10">
          <QRCodeSVG value={code} size={224} level="M" title="QR de pareamento do WhatsApp" />
        </div>
      ) : (
        <div className="flex size-56 flex-col items-center justify-center gap-2 rounded-xl border border-dashed text-center">
          {displayStream === "error" ? (
            <>
              <TriangleAlert className="size-6 text-amber-500" aria-hidden />
              <p className="px-4 text-sm text-muted-foreground">
                Não foi possível abrir o stream de QR. Verifique a configuração do bridge.
              </p>
            </>
          ) : (
            <>
              <RefreshCw className="size-6 animate-spin text-muted-foreground" aria-hidden />
              <p className="text-sm text-muted-foreground">Aguardando QR do bridge…</p>
            </>
          )}
        </div>
      )}

      <div className="flex flex-wrap items-center justify-center gap-2 text-xs text-muted-foreground">
        <Badge variant="outline" className="font-mono">
          status: {displayStatus}
        </Badge>
        <Badge variant="outline" className="font-mono">
          stream: {displayStream}
        </Badge>
        {displayStream === "error" || displayStream === "closed" ? (
          <Button
            type="button"
            variant="outline"
            size="xs"
            onClick={() => {
              sessionStorage.removeItem(`panel:qr-refreshed:${tenantId}`);
              setCode(null);
              setStream("live");
              setAttempt((value) => value + 1);
            }}
          >
            <RefreshCw aria-hidden />
            Reconectar
          </Button>
        ) : null}
      </div>

      <p className="max-w-md text-center text-sm text-muted-foreground">
        Abra o WhatsApp no celular → <span className="font-medium">Dispositivos conectados</span> →{" "}
        <span className="font-medium">Conectar dispositivo</span> e escaneie o código. O código se renova
        automaticamente enquanto o stream estiver aberto.
      </p>
    </div>
  );
}
