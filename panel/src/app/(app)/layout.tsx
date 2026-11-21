import { connection } from "next/server";
import { redirect } from "next/navigation";

import { AppShell } from "@/components/app-shell";
import { SignOutButton } from "@/components/auth/sign-out-button";
import { RefreshButton } from "@/components/refresh-button";
import { StatusBadge } from "@/components/status-badge";
import { ThemeToggle } from "@/components/theme-toggle";
import { Badge } from "@/components/ui/badge";
import { isEmailAllowed } from "@/lib/access";
import { loadStatus, loadTenants } from "@/lib/api";
import { BRIDGE_URL } from "@/lib/bridge";
import { ensureProvisionedTenant } from "@/lib/provisioning";
import { getTenantContext, requireSession } from "@/lib/session";

export const instant = false;

export default async function AppLayout({ children }: { children: React.ReactNode }) {
  await connection();

  const session = await requireSession();
  if (!isEmailAllowed(session.user.email)) {
    redirect("/login");
  }
  const context = (await getTenantContext()) ?? (await ensureProvisionedTenant(session));
  if (!context) {
    redirect("/onboarding");
  }
  const statusResult = await loadStatus();
  const tenantsResult = context.isAdmin ? await loadTenants() : null;
  const tenants = tenantsResult?.ok ? tenantsResult.data.map((tenant) => ({ id: tenant.id, label: tenant.label })) : [];
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
      tenants={tenants}
    >
      {children}
    </AppShell>
  );
}
