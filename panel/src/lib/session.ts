import { and, asc, eq } from "drizzle-orm";
import { headers } from "next/headers";
import { redirect } from "next/navigation";

import { db } from "@/db";
import { member, tenantCredentials, user } from "@/db/schema";
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
  let organizationId = session.session.activeOrganizationId ?? null;
  if (!organizationId) {
    const membership = await db
      .select({ organizationId: member.organizationId })
      .from(member)
      .where(eq(member.userId, session.user.id))
      .limit(1);
    organizationId = membership[0]?.organizationId ?? null;
  }
  if (!organizationId) {
    return null;
  }
  const credential = await db
    .select()
    .from(tenantCredentials)
    .where(eq(tenantCredentials.organizationId, organizationId))
    .limit(1);
  if (!credential[0]) {
    return null;
  }
  const membership = await db
    .select({ role: member.role })
    .from(member)
    .where(and(eq(member.organizationId, organizationId), eq(member.userId, session.user.id)))
    .limit(1);
  const organizationRole = membership[0]?.role ?? "member";
  const isAdmin = await isPanelAdmin(session.user.email, session.user.id);
  const role: PanelRole = isAdmin ? "admin" : organizationRole === "owner" ? "owner" : "member";
  return {
    userId: session.user.id,
    email: session.user.email,
    name: session.user.name,
    organizationId,
    tenantId: credential[0].tenantId,
    token: decryptSecret(credential[0].encryptedToken),
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
