"use client";

import { useRouter } from "next/navigation";
import { useTransition } from "react";

import { cn } from "@/lib/utils";

type TenantOption = {
  id: string;
  label: string;
};

export function TenantSwitcher({
  current,
  tenants,
  className,
}: {
  current: string;
  tenants: TenantOption[];
  className?: string;
}) {
  const router = useRouter();
  const [pending, startTransition] = useTransition();

  async function change(event: React.ChangeEvent<HTMLSelectElement>) {
    const tenant = event.target.value;
    if (!tenant || tenant === current) {
      return;
    }
    const response = await fetch("/api/panel/tenant", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ tenant }),
    });
    if (!response.ok) {
      return;
    }
    startTransition(() => router.refresh());
  }

  return (
    <select
      aria-label="Trocar conta"
      value={current}
      onChange={change}
      disabled={pending}
      className={cn(
        "mt-2 h-8 w-full rounded-md border border-input bg-card px-2 text-xs text-foreground outline-none focus-visible:ring-3 focus-visible:ring-ring/50 disabled:opacity-60",
        className,
      )}
    >
      {tenants.map((tenant) => (
        <option key={tenant.id} value={tenant.id}>
          {tenant.label ? `${tenant.label} (${tenant.id})` : tenant.id}
        </option>
      ))}
    </select>
  );
}
