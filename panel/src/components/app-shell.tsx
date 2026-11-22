"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import type { ReactNode } from "react";
import {
  Activity,
  Building2,
  KeyRound,
  LayoutDashboard,
  MessagesSquare,
  QrCode,
  Settings,
  Webhook,
} from "lucide-react";

import { STATUS_DOT_CLASSES, STATUS_LABELS, type TenantStatusValue } from "@/lib/status";
import type { PanelRole } from "@/lib/session";
import { initials } from "@/lib/format";
import { Avatar, AvatarFallback } from "@/components/ui/avatar";
import { BrandMark } from "@/components/brand-mark";
import { cn } from "@/lib/utils";
import { SignOutButton } from "@/components/auth/sign-out-button";
import { TenantSwitcher } from "@/components/tenant-switcher";

type NavItem = {
  href: string;
  label: string;
  icon: typeof LayoutDashboard;
  admin?: boolean;
};

const NAV_GROUPS: { label: string; items: NavItem[] }[] = [
  {
    label: "Operação",
    items: [
      { href: "/", label: "Visão geral", icon: LayoutDashboard },
      { href: "/chats", label: "Conversas", icon: MessagesSquare },
      { href: "/events", label: "Eventos", icon: Activity },
    ],
  },
  {
    label: "Automação",
    items: [
      { href: "/webhooks", label: "Webhooks", icon: Webhook },
    ],
  },
  {
    label: "Acesso",
    items: [
      { href: "/keys", label: "Chaves", icon: KeyRound },
      { href: "/tenants", label: "Tenants", icon: Building2, admin: true },
    ],
  },
  {
    label: "Sessão",
    items: [{ href: "/settings", label: "Pareamento", icon: QrCode }],
  },
];

function isActive(pathname: string, href: string): boolean {
  if (href === "/") {
    return pathname === "/";
  }
  return pathname === href || pathname.startsWith(`${href}/`);
}

function Brand({ compact = false }: { compact?: boolean }) {
  return (
    <Link href="/" className="flex min-w-0 items-center gap-2.5">
      <span className="flex size-9 shrink-0 items-center justify-center rounded-[0.7rem] bg-sidebar-primary text-highlight shadow-[inset_0_1px_0_rgb(255_255_255/0.18)]">
        <BrandMark />
      </span>
      {compact ? null : (
        <span className="flex min-w-0 flex-col leading-tight">
          <span className="truncate font-[family-name:var(--font-sora)] text-[13px] font-semibold tracking-tight">
            WhatsApp Bridge
          </span>
          <span className="truncate text-[10px] font-medium tracking-[0.14em] text-muted-foreground uppercase">
            painel operacional
          </span>
        </span>
      )}
    </Link>
  );
}

function NavLink({ item, active }: { item: NavItem; active: boolean }) {
  return (
    <Link
      href={item.href}
      aria-current={active ? "page" : undefined}
      className={cn(
        "group flex items-center gap-2.5 rounded-lg px-2.5 py-2 text-[13px] font-medium transition-colors",
        active
          ? "bg-sidebar-accent text-sidebar-accent-foreground"
          : "text-sidebar-foreground/70 hover:bg-sidebar-accent/60 hover:text-sidebar-accent-foreground",
      )}
    >
      <item.icon
        className={cn("size-4 shrink-0", active ? "text-sidebar-accent-foreground" : "text-muted-foreground")}
        aria-hidden
      />
      <span className="flex-1">{item.label}</span>
      {item.admin ? (
        <span className="rounded border border-highlight/40 bg-highlight/15 px-1 py-0.5 text-[9px] font-semibold tracking-[0.12em] text-sidebar-accent-foreground uppercase">
          admin
        </span>
      ) : null}
    </Link>
  );
}

function TenantCard({
  tenant,
  status,
  role,
  tenants,
}: {
  tenant: string;
  status: TenantStatusValue | null;
  role: PanelRole;
  tenants: { id: string; label: string }[];
}) {
  const content = (
    <div className="flex items-center gap-2.5">
      <span className="flex size-8 shrink-0 items-center justify-center rounded-lg bg-accent text-accent-foreground">
        <Building2 className="size-4" aria-hidden />
      </span>
      <div className="min-w-0 flex-1">
        <p className="flex items-center gap-1 text-[10px] font-semibold tracking-[0.12em] text-muted-foreground uppercase">
          Tenant
          <span
            className={cn(
              "rounded px-1 py-0.5 text-[9px] tracking-[0.08em]",
              role === "admin"
                ? "bg-highlight/20 text-highlight-foreground"
                : "bg-muted text-muted-foreground",
            )}
          >
            {role}
          </span>
        </p>
        <p className="truncate font-mono text-xs font-medium">{tenant}</p>
      </div>
      {status ? (
        <span
          className={cn("size-2 shrink-0 rounded-full", STATUS_DOT_CLASSES[status])}
          title={STATUS_LABELS[status]}
          aria-label={STATUS_LABELS[status]}
        />
      ) : null}
    </div>
  );

  if (role === "admin" && tenants.length > 1) {
    return (
      <div className="mx-3 mt-3 rounded-lg border border-sidebar-border bg-background p-3">
        {content}
        <TenantSwitcher current={tenant} tenants={tenants} />
        <Link
          href="/tenants"
          className="mt-1.5 block text-[10px] text-muted-foreground underline-offset-4 hover:text-foreground hover:underline"
        >
          Gerenciar tenants
        </Link>
      </div>
    );
  }

  return (
    <Link
      href="/tenants"
      className="mx-3 mt-3 block rounded-lg border border-sidebar-border bg-background p-3 transition-colors hover:bg-muted"
    >
      {content}
    </Link>
  );
}

export function AppShell({
  header,
  children,
  tenant,
  status,
  bridgeUrl,
  isAdmin = false,
  role = "member",
  showBanner = false,
  userName = "",
  userEmail = "",
  tenants = [],
}: {
  header: ReactNode;
  children: ReactNode;
  tenant: string;
  status: TenantStatusValue | null;
  bridgeUrl: string;
  isAdmin?: boolean;
  role?: PanelRole;
  showBanner?: boolean;
  userName?: string;
  userEmail?: string;
  tenants?: { id: string; label: string }[];
}) {
  const pathname = usePathname();
  const host = bridgeUrl.replace(/^https?:\/\//, "").replace(/\/api\/v1\/?$/, "");
  const navGroups = NAV_GROUPS.map((group) => ({
    ...group,
    items: group.items.filter((item) => !item.admin || isAdmin),
  })).filter((group) => group.items.length > 0);

  return (
    <div className="min-h-svh bg-background">
      {showBanner ? (
        <div className="fixed inset-x-0 top-0 z-50 flex h-10 items-center px-2 sm:px-4">
          <div className="flex min-w-0 flex-1 items-center justify-center gap-2 rounded-md bg-banner px-3 py-2 text-xs font-medium text-banner-foreground">
            <Activity className="size-3.5 shrink-0" aria-hidden />
            <span className="truncate">
              Ambiente local · API em <span className="font-mono">{host}</span>
            </span>
            <Link href="/settings" className="shrink-0 underline underline-offset-4 hover:no-underline">
              Configurar
            </Link>
          </div>
        </div>
      ) : null}

      <aside
        className={cn(
          "fixed bottom-0 left-0 z-40 hidden w-64 flex-col border-r border-sidebar-border bg-sidebar text-sidebar-foreground lg:flex",
          showBanner ? "top-10" : "top-0",
        )}
      >
        <div className="flex h-16 items-center border-b border-sidebar-border px-4">
          <Brand />
        </div>
        <TenantCard tenant={tenant} status={status} role={role} tenants={tenants} />
        <nav className="flex-1 overflow-y-auto px-3 py-4" aria-label="Navegação principal">
          {navGroups.map((group) => (
            <div key={group.label} className="mb-5 last:mb-0">
              <p className="mb-1.5 px-2.5 text-[10px] font-semibold tracking-[0.16em] text-sidebar-foreground/55 uppercase">
                {group.label}
              </p>
              <div className="space-y-0.5">
                {group.items.map((item) => (
                  <NavLink key={item.href} item={item} active={isActive(pathname, item.href)} />
                ))}
              </div>
            </div>
          ))}
        </nav>
        <div className="border-t border-sidebar-border p-3">
          <Link
            href="/settings"
            className="flex items-center gap-2 rounded-lg px-2.5 py-2 text-xs text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
          >
            <Settings className="size-3.5" aria-hidden />
            Configuração do tenant
          </Link>
          {userEmail ? (
            <div className="mt-2 flex items-center gap-2 rounded-lg px-1.5 py-1.5">
              <Avatar className="size-8 shrink-0">
                <AvatarFallback className="bg-highlight text-[11px] font-semibold text-highlight-foreground">
                  {initials(userName || userEmail)}
                </AvatarFallback>
              </Avatar>
              <div className="min-w-0 flex-1">
                <p className="truncate text-xs font-medium">{userName || "Conta"}</p>
                <p className="truncate text-[10px] text-muted-foreground">{userEmail}</p>
              </div>
              <SignOutButton compact className="shrink-0" />
            </div>
          ) : (
            <SignOutButton className="mt-1 w-full justify-start text-muted-foreground hover:bg-muted hover:text-foreground" />
          )}
        </div>
      </aside>

      <div className={cn("flex min-h-svh flex-col lg:pl-64", showBanner && "pt-10")}>
        <header
          className={cn(
            "sticky z-30 border-b bg-background",
            showBanner ? "top-10" : "top-0",
          )}
        >
          <div className="flex h-14 items-center gap-3 px-4 sm:px-6">
            <div className="lg:hidden">
              <Brand compact />
            </div>
            <div className="ml-auto flex min-w-0 items-center gap-1.5 sm:gap-2">{header}</div>
          </div>
          <nav className="flex gap-1 overflow-x-auto border-t px-3 py-2 lg:hidden" aria-label="Navegação principal">
            {navGroups.flatMap((group) => group.items).map((item) => {
              const active = isActive(pathname, item.href);
              return (
                <Link
                  key={item.href}
                  href={item.href}
                  aria-current={active ? "page" : undefined}
                  className={cn(
                    "flex shrink-0 items-center gap-1.5 rounded-lg px-2.5 py-1.5 text-xs font-medium whitespace-nowrap transition-colors",
                    active
                      ? "bg-highlight text-highlight-foreground"
                      : "text-muted-foreground hover:bg-muted hover:text-foreground",
                  )}
                >
                  <item.icon className="size-3.5" aria-hidden />
                  {item.label}
                </Link>
              );
            })}
          </nav>
        </header>
        <main className="flex-1 px-4 py-6 sm:px-6 lg:px-8">
          <div className="mx-auto w-full max-w-6xl">{children}</div>
        </main>
      </div>
    </div>
  );
}
