import { and, eq } from "drizzle-orm";
import { randomUUID } from "node:crypto";

import { db } from "@/db";
import { member, organization, tenantCredentials } from "@/db/schema";
import { encryptSecret } from "@/lib/crypto";

const BRIDGE_URL = (process.env.BRIDGE_URL ?? "http://127.0.0.1:8080/api/v1").replace(/\/$/, "");

type Envelope<T> = { success?: boolean; data?: T; error?: { code?: string; message?: string } | null };
type TenantCreation = { tenant: { id: string; label: string }; token: string };

export type ConnectResult = { ok: true; tenantId: string } | { ok: false; error: string };

async function bridgeRequest<T>(path: string, token: string, init?: RequestInit): Promise<{ ok: true; data: T } | { ok: false; error: string }> {
  try {
    const response = await fetch(`${BRIDGE_URL}${path}`, {
      ...init,
      headers: {
        Authorization: `Bearer ${token}`,
        ...(init?.body ? { "Content-Type": "application/json" } : {}),
        ...init?.headers,
      },
      cache: "no-store",
    });
    const payload = (await response.json().catch(() => null)) as Envelope<T> | null;
    if (!response.ok || !payload?.success || !payload.data) {
      return { ok: false, error: payload?.error?.message ?? `bridge respondeu ${response.status}` };
    }
    return { ok: true, data: payload.data };
  } catch (error) {
    return { ok: false, error: error instanceof Error ? error.message : "falha de rede" };
  }
}

async function linkTenant(userId: string, tenantId: string, token: string, label: string): Promise<ConnectResult> {
  const existing = await db.select().from(organization).where(eq(organization.slug, tenantId)).limit(1);
  const organizationId = existing[0]?.id ?? randomUUID();

  if (!existing[0]) {
    await db.insert(organization).values({
      id: organizationId,
      name: label,
      slug: tenantId,
      createdAt: new Date(),
    });
  }

  const membership = await db
    .select()
    .from(member)
    .where(and(eq(member.organizationId, organizationId), eq(member.userId, userId)))
    .limit(1);
  if (!membership[0]) {
    const existingMembers = await db
      .select({ id: member.id })
      .from(member)
      .where(eq(member.organizationId, organizationId));
    await db.insert(member).values({
      id: randomUUID(),
      organizationId,
      userId,
      role: existingMembers.length === 0 ? "owner" : "member",
      createdAt: new Date(),
    });
  }

  const credential = await db
    .select()
    .from(tenantCredentials)
    .where(eq(tenantCredentials.organizationId, organizationId))
    .limit(1);
  const encryptedToken = encryptSecret(token);
  if (credential[0]) {
    await db
      .update(tenantCredentials)
      .set({ tenantId, encryptedToken })
      .where(eq(tenantCredentials.organizationId, organizationId));
  } else {
    await db.insert(tenantCredentials).values({
      id: randomUUID(),
      organizationId,
      tenantId,
      encryptedToken,
    });
  }
  return { ok: true, tenantId };
}

export async function connectExistingTenant(userId: string, tenantId: string, token: string, label: string): Promise<ConnectResult> {
  const status = await bridgeRequest<{ tenant: { id: string } }>(`/tenants/${encodeURIComponent(tenantId)}/status`, token);
  if (!status.ok) {
    return { ok: false, error: `Key inválida para o tenant ${tenantId}: ${status.error}` };
  }
  return linkTenant(userId, tenantId, token, label);
}

export async function createAndLinkTenant(userId: string, tenantId: string, label: string): Promise<ConnectResult> {
  const adminToken = process.env.BRIDGE_ADMIN_TOKEN;
  if (!adminToken) {
    return { ok: false, error: "BRIDGE_ADMIN_TOKEN não está configurado no painel." };
  }
  const created = await bridgeRequest<TenantCreation>("/tenants", adminToken, {
    method: "POST",
    body: JSON.stringify({ id: tenantId, label }),
  });
  if (!created.ok) {
    return { ok: false, error: created.error };
  }
  return linkTenant(userId, tenantId, created.data.token, label);
}

export function isValidTenantId(value: string): boolean {
  return /^[a-z0-9][a-z0-9-]{1,31}$/.test(value);
}
