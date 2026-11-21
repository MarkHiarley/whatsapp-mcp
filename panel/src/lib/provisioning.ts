import { randomBytes } from "node:crypto";

import { isEmailAllowed } from "@/lib/access";
import { createAndLinkTenant, isValidTenantId } from "@/lib/onboarding";
import { getTenantContext, type SessionLike, type TenantContext } from "@/lib/session";

const inflight = new Map<string, Promise<TenantContext | null>>();

function slugFromSession(session: SessionLike): string {
  const source = (session.user.email.split("@")[0] || session.user.name || "conta")
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "")
    .slice(0, 20);
  return source || "conta";
}

function candidateTenantID(session: SessionLike): string {
  return `${slugFromSession(session)}-${randomBytes(3).toString("hex")}`;
}

/**
 * Creates one tenant for a user on first login, so the panel opens with a
 * usable session instead of an empty onboarding screen. Admin token required.
 */
export async function ensureProvisionedTenant(session: SessionLike): Promise<TenantContext | null> {
  if (!isEmailAllowed(session.user.email)) {
    return null;
  }
  const existing = await getTenantContext();
  if (existing) {
    return existing;
  }

  const pending = inflight.get(session.user.id);
  if (pending) {
    return pending;
  }

  const task = (async () => {
    if (!process.env.BRIDGE_ADMIN_TOKEN) {
      return null;
    }
    const label = session.user.name?.trim() || session.user.email.split("@")[0] || "Conta";
    for (let attempt = 0; attempt < 3; attempt += 1) {
      const tenantId = candidateTenantID(session);
      if (!isValidTenantId(tenantId)) {
        continue;
      }
      const created = await createAndLinkTenant(session.user.id, tenantId, label);
      if (!created.ok) {
        if (created.error.includes("CONFLICT") || created.error.includes("already exists")) {
          continue;
        }
        return null;
      }
      return getTenantContext();
    }
    return null;
  })().finally(() => {
    inflight.delete(session.user.id);
  });

  inflight.set(session.user.id, task);
  return task;
}
