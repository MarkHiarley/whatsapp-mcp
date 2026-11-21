import { cookies, headers } from "next/headers";
import { redirect } from "next/navigation";

import { and, asc, eq } from "drizzle-orm";

import { db } from "@/db";
import { member, tenantCredentials, user } from "@/db/schema";
import { isEmailAllowed } from "@/lib/access";
import { auth } from "@/lib/auth";
import { decryptSecret } from "@/lib/crypto";

export type SessionLike = typeof auth.$Infer.Session;

export type PanelRole = "admin" | "owner" | "member";

export type TenantContext = {
  userId: string;
  email: string;
  name: string;
  organizationId: string;
  tenantId: string;
  token: string;
  isAdmin: boolean;
  role: PanelRole;
  organizationRole: string;
};

export const TENANT_COOKIE = "panel_tenant";

export async function getSession(): Promise<SessionLike | null> {
  return auth.api.getSession({ headers: await headers() });
}

export async function isPanelAdmin(email: string, userId: string): Promise<boolean> {
  const configured = [process.env.PRIMER_ADMIN, ...(process.env.PANEL_ADMIN_EMAILS ?? "").split(",")]
    .map((entry) => entry?.trim().toLowerCase())
    .filter((entry): entry is string => Boolean(entry));
  if (configured.length > 0) {
    return configured.includes(email.toLowerCase());
  }
  const first = await db.select({ id: user.id }).from(user).orderBy(asc(user.createdAt)).limit(1);
  return first[0]?.id === userId;
}

async function contextFromSession(session: SessionLike): Promise<TenantContext | null> {
  if (!isEmailAllowed(session.user.email)) {
    return null;
  }
  const isAdmin = await isPanelAdmin(session.user.email, session.user.id);
  const adminToken = process.env.BRIDGE_ADMIN_TOKEN;
  const cookieStore = await cookies();
  const selectedTenant = isAdmin ? cookieStore.get(TENANT_COOKIE)?.value : undefined;

  let organizationId = session.session.activeOrganizationId ?? null;
  if (!organizationId) {
    const membership = await db
      .select({ organizationId: member.organizationId })
      .from(member)
      .where(eq(member.userId, session.user.id))
      .limit(1);
    organizationId = membership[0]?.organizationId ?? null;
  }

  const credential = organizationId
    ? await db
        .select()
        .from(tenantCredentials)
        .where(eq(tenantCredentials.organizationId, organizationId))
        .limit(1)
    : [];

  const organizationRole = organizationId
    ? (
        await db
          .select({ role: member.role })
          .from(member)
          .where(and(eq(member.organizationId, organizationId), eq(member.userId, session.user.id)))
          .limit(1)
      )[0]?.role ?? "member"
    : "member";

  const tenantId = isAdmin && selectedTenant ? selectedTenant : credential[0]?.tenantId;
  const token = isAdmin && adminToken ? adminToken : credential[0] ? decryptSecret(credential[0].encryptedToken) : null;

  if (!tenantId || !token) {
    return null;
  }

  const role: PanelRole = isAdmin ? "admin" : organizationRole === "owner" ? "owner" : "member";
  return {
    userId: session.user.id,
    email: session.user.email,
    name: session.user.name,
    organizationId: organizationId ?? "",
    tenantId,
    token,
    isAdmin,
    role,
    organizationRole,
  };
}

export async function getTenantContext(): Promise<TenantContext | null> {
  const session = await getSession();
  if (!session) {
    return null;
  }
  return contextFromSession(session);
}

export async function requireSession(): Promise<SessionLike> {
  const session = await getSession();
  if (!session) {
    redirect("/login");
  }
  return session;
}

export async function requireTenantContext(): Promise<TenantContext> {
  const session = await requireSession();
  const context = await contextFromSession(session);
  if (!context) {
    redirect("/onboarding");
  }
  return context;
}
