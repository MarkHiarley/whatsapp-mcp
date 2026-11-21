"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { cn } from "@/lib/utils";

type Mode = "connect" | "create";

export function OnboardingForm({ isAdmin }: { isAdmin: boolean }) {
  const router = useRouter();
  const [mode, setMode] = useState<Mode>("connect");
  const [tenantId, setTenantId] = useState("");
  const [label, setLabel] = useState("");
  const [key, setKey] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [pending, setPending] = useState(false);

  async function submit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setPending(true);
    setError(null);
    const endpoint = mode === "connect" ? "/api/onboarding/connect" : "/api/onboarding/create";
    const body = mode === "connect" ? { tenantId, key, label } : { tenantId, label };
    const response = await fetch(endpoint, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
    const payload = (await response.json().catch(() => null)) as { ok?: boolean; error?: string } | null;
    if (!response.ok || !payload?.ok) {
      setError(payload?.error ?? "Não foi possível concluir a conexão.");
      setPending(false);
      return;
    }
    router.push("/");
    router.refresh();
  }

  function switchMode(next: Mode) {
    setMode(next);
    setError(null);
  }

  return (
    <div>
      <div className="flex gap-1 rounded-lg border p-1" role="tablist" aria-label="Forma de conectar">
        <button
          type="button"
          role="tab"
          aria-selected={mode === "connect"}
          onClick={() => switchMode("connect")}
          className={cn(
            "flex-1 rounded-md px-3 py-1.5 text-xs font-medium transition-colors",
            mode === "connect" ? "bg-primary text-primary-foreground" : "text-muted-foreground hover:bg-muted",
          )}
        >
          Conectar existente
        </button>
        {isAdmin ? (
          <button
            type="button"
            role="tab"
            aria-selected={mode === "create"}
            onClick={() => switchMode("create")}
            className={cn(
              "flex-1 rounded-md px-3 py-1.5 text-xs font-medium transition-colors",
              mode === "create" ? "bg-primary text-primary-foreground" : "text-muted-foreground hover:bg-muted",
            )}
          >
            Criar tenant
          </button>
        ) : null}
      </div>

      <form className="mt-4 space-y-3" onSubmit={submit}>
        <div className="space-y-1.5">
          <Label htmlFor="tenant-id">Tenant</Label>
          <Input
            id="tenant-id"
            value={tenantId}
            onChange={(event) => setTenantId(event.target.value)}
            placeholder="cliente-x"
            pattern="[a-z0-9][a-z0-9-]{1,31}"
            required
            autoComplete="off"
          />
          <p className="text-[11px] text-muted-foreground">Minúsculas, números e hífen; 2 a 32 caracteres.</p>
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="tenant-label">Nome</Label>
          <Input
            id="tenant-label"
            value={label}
            onChange={(event) => setLabel(event.target.value)}
            placeholder="Cliente X"
            required={mode === "create"}
            autoComplete="off"
          />
        </div>

        {mode === "connect" ? (
          <div className="space-y-1.5">
            <Label htmlFor="tenant-key">Key do tenant</Label>
            <Input
              id="tenant-key"
              type="password"
              value={key}
              onChange={(event) => setKey(event.target.value)}
              placeholder="token retornado pelo bridge"
              required
              autoComplete="off"
            />
            <p className="text-[11px] text-muted-foreground">
              A key é validada no bridge e guardada cifrada no painel.
            </p>
          </div>
        ) : (
          <p className="text-[11px] text-muted-foreground">
            O tenant é criado pelo admin token do bridge e a key retornada fica cifrada no painel.
          </p>
        )}

        {error ? <p className="text-xs text-destructive">{error}</p> : null}

        <Button type="submit" className="w-full" disabled={pending}>
          {pending ? "Conectando…" : mode === "connect" ? "Conectar tenant" : "Criar e conectar"}
        </Button>
      </form>
    </div>
  );
}
