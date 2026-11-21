import { connection } from "next/server";

import { AppShell } from "@/components/app-shell";
import { SignOutButton } from "@/components/auth/sign-out-button";
import { RefreshButton } from "@/components/refresh-button";
import { StatusBadge } from "@/components/status-badge";
import { ThemeToggle } from "@/components/theme-toggle";
import { Badge } from "@/components/ui/badge";
import { loadStatus } from "@/lib/api";
import { BRIDGE_URL } from "@/lib/bridge";
import { requireTenantContext } from "@/lib/session";

export const instant = false;

export default async function AppLayout({ children }: { children: React.ReactNode }) {
  await connection();

  const context = await requireTenantContext();
  const statusResult = await loadStatus();
  const status = statusResult.ok ? statusResult.data.status : null;
  const statusError = statusResult.ok ? undefined : statusResult.error;

  const header = (
    <>
      {status ? (
        <StatusBadge status={status} />
      ) : (
        <Badge variant="destructive" title={statusError}>
          status indisponível
        </Badge>
      )}
      <RefreshButton />
      <ThemeToggle />
      <SignOutButton compact className="lg:hidden" />
    </>
  );

  return (
    <AppShell
      header={header}
      tenant={context.tenantId}
      status={status}
      bridgeUrl={BRIDGE_URL}
      isAdmin={context.isAdmin}
      role={context.role}
      showBanner={process.env.NODE_ENV !== "production"}
      userName={context.name}
      userEmail={context.email}
    >
      {children}
    </AppShell>
  );
}
