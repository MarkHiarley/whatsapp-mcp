import { createHash, randomBytes, randomUUID } from "node:crypto";

import { and, desc, eq } from "drizzle-orm";

import { db } from "@/db";
import { mcpToken, tenantCredentials } from "@/db/schema";
import { decryptSecret, encryptSecret } from "@/lib/crypto";

export const MCP_SCOPES = ["read", "write"] as const;
export type McpScope = (typeof MCP_SCOPES)[number];

export type CreatedMcpToken = {
  id: string;
  name: string;
  tenantId: string;
  scopes: McpScope[];
  token: string;
  createdAt: Date;
};

function hashToken(token: string): string {
  return createHash("sha256").update(token, "utf8").digest("hex");
}

function normalizeScopes(scopes: string[]): McpScope[] {
  const requested = scopes.filter((scope): scope is McpScope => MCP_SCOPES.includes(scope as McpScope));
  return Array.from(new Set<McpScope>(["read", ...requested]));
}

async function verifyBridgeCredential(tenantId: string, apiToken: string): Promise<void> {
  const base = process.env.BRIDGE_URL?.trim().replace(/\/+$/, "");
  if (!base) {
    throw new Error("BRIDGE_NOT_CONFIGURED");
  }
  let response: Response;
  try {
    response = await fetch(`${base}/tenants/${encodeURIComponent(tenantId)}/status`, {
      headers: { Authorization: `Bearer ${apiToken}` },
      cache: "no-store",
      signal: AbortSignal.timeout(5000),
    });
  } catch {
    throw new Error("BRIDGE_UNAVAILABLE");
  }
  if (response.status === 404) {
    throw new Error("TENANT_NOT_FOUND");
  }
  if (response.status === 401 || response.status === 403) {
    throw new Error("TENANT_KEY_INVALID");
  }
  if (!response.ok) {
    throw new Error("BRIDGE_UNAVAILABLE");
  }
}

async function createBridgeKey(tenantId: string, name: string): Promise<{ encrypted: string; keyId: string }> {
  const base = process.env.BRIDGE_URL?.trim().replace(/\/+$/, "");
  const admin = process.env.BRIDGE_ADMIN_TOKEN;
  if (!base) {
    throw new Error("BRIDGE_NOT_CONFIGURED");
  }
  if (!admin) {
    throw new Error("BRIDGE_ADMIN_NOT_CONFIGURED");
  }
  let response: Response;
  try {
    response = await fetch(`${base}/tenants/${encodeURIComponent(tenantId)}/keys`, {
      method: "POST",
      headers: { Authorization: `Bearer ${admin}`, "Content-Type": "application/json" },
      body: JSON.stringify({ label: `mcp-${name}`.slice(0, 80) }),
      cache: "no-store",
      signal: AbortSignal.timeout(5000),
    });
  } catch {
    throw new Error("BRIDGE_UNAVAILABLE");
  }
  if (response.status === 404) {
    throw new Error("TENANT_NOT_FOUND");
  }
  if (!response.ok) {
    throw new Error("BRIDGE_KEY_CREATE_FAILED");
  }
  const payload = (await response.json()) as { data?: { key?: { id?: unknown }; token?: unknown } };
  const keyId = payload.data?.key?.id;
  const keyToken = payload.data?.token;
  if (typeof keyId !== "string" || typeof keyToken !== "string") {
    throw new Error("BRIDGE_KEY_CREATE_FAILED");
  }
  return { encrypted: encryptSecret(keyToken), keyId };
}

async function revokeBridgeKey(tenantId: string, keyId: string): Promise<void> {
  const base = process.env.BRIDGE_URL?.trim().replace(/\/+$/, "");
  const admin = process.env.BRIDGE_ADMIN_TOKEN;
  if (!base || !admin) {
    return;
  }
  try {
    await fetch(`${base}/tenants/${encodeURIComponent(tenantId)}/keys/${encodeURIComponent(keyId)}`, {
      method: "DELETE",
      headers: { Authorization: `Bearer ${admin}` },
      cache: "no-store",
      signal: AbortSignal.timeout(5000),
    });
  } catch {
    // Revocation is best-effort; the MCP token is already invalid in the panel.
  }
}

export async function createMcpToken(input: {
  organizationId: string;
  tenantId: string;
  userId: string;
  name: string;
  scopes: string[];
  isAdmin: boolean;
}): Promise<CreatedMcpToken> {
  const credential = await db
    .select()
    .from(tenantCredentials)
    .where(eq(tenantCredentials.organizationId, input.organizationId))
    .limit(1);
  const tenantId = input.tenantId;
  let encryptedToken: string;
  let bridgeKeyId: string | null = null;

  if (credential[0] && credential[0].tenantId === tenantId) {
    encryptedToken = credential[0].encryptedToken;
  } else {
    if (!input.isAdmin) {
      throw new Error("TENANT_FORBIDDEN");
    }
    const key = await createBridgeKey(tenantId, input.name);
    encryptedToken = key.encrypted;
    bridgeKeyId = key.keyId;
  }
  await verifyBridgeCredential(tenantId, decryptSecret(encryptedToken));

  const token = `mcp_${randomBytes(32).toString("base64url")}`;
  const scopes = normalizeScopes(input.scopes);
  const createdAt = new Date();
  const id = randomUUID();
  try {
    await db.insert(mcpToken).values({
      id,
      organizationId: input.organizationId,
      tenantId,
      name: input.name.trim().slice(0, 80) || "Agente",
      tokenHash: hashToken(token),
      scopes: JSON.stringify(scopes),
      encryptedToken,
      bridgeKeyId,
      createdBy: input.userId,
      createdAt,
    });
  } catch (error) {
    if (bridgeKeyId) {
      await revokeBridgeKey(tenantId, bridgeKeyId);
    }
    throw error;
  }
  return { id, name: input.name.trim().slice(0, 80) || "Agente", tenantId, scopes, token, createdAt };
}

export async function listMcpTokens(organizationId: string, tenantId: string) {
  return db
    .select({
      id: mcpToken.id,
      name: mcpToken.name,
      scopes: mcpToken.scopes,
      tenantId: mcpToken.tenantId,
      createdAt: mcpToken.createdAt,
      lastUsedAt: mcpToken.lastUsedAt,
      revokedAt: mcpToken.revokedAt,
    })
    .from(mcpToken)
    .where(and(eq(mcpToken.organizationId, organizationId), eq(mcpToken.tenantId, tenantId)))
    .orderBy(desc(mcpToken.createdAt));
}

export async function revokeMcpToken(organizationId: string, tenantId: string, id: string): Promise<boolean> {
  const rows = await db
    .select({ tenantId: mcpToken.tenantId, bridgeKeyId: mcpToken.bridgeKeyId })
    .from(mcpToken)
    .where(and(eq(mcpToken.organizationId, organizationId), eq(mcpToken.tenantId, tenantId), eq(mcpToken.id, id)))
    .limit(1);
  if (!rows[0]) {
    return false;
  }
  const result = await db
    .update(mcpToken)
    .set({ revokedAt: new Date() })
    .where(and(eq(mcpToken.organizationId, organizationId), eq(mcpToken.tenantId, tenantId), eq(mcpToken.id, id)))
    .returning({ id: mcpToken.id });
  if (result.length > 0 && rows[0].bridgeKeyId) {
    await revokeBridgeKey(rows[0].tenantId, rows[0].bridgeKeyId);
  }
  return result.length > 0;
}

export async function introspectMcpToken(rawToken: string): Promise<{
  tenant: string;
  apiToken: string;
  scopes: McpScope[];
  clientId: string;
  subject: string;
} | null> {
  const rows = await db
    .select()
    .from(mcpToken)
    .where(eq(mcpToken.tokenHash, hashToken(rawToken)))
    .limit(1);
  const found = rows[0];
  if (!found || found.revokedAt) {
    return null;
  }

  let apiToken: string;
  if (found.encryptedToken) {
    apiToken = decryptSecret(found.encryptedToken);
  } else {
    const credential = await db
      .select()
      .from(tenantCredentials)
      .where(and(eq(tenantCredentials.organizationId, found.organizationId), eq(tenantCredentials.tenantId, found.tenantId)))
      .limit(1);
    if (!credential[0]) {
      return null;
    }
    apiToken = decryptSecret(credential[0].encryptedToken);
  }

  await db.update(mcpToken).set({ lastUsedAt: new Date() }).where(eq(mcpToken.id, found.id));
  let scopes: McpScope[];
  try {
    scopes = normalizeScopes(JSON.parse(found.scopes) as string[]);
  } catch {
    scopes = ["read"];
  }
  return {
    tenant: found.tenantId,
    apiToken,
    scopes,
    clientId: found.name,
    subject: found.tenantId,
  };
}
