import { createHash, randomBytes, randomUUID } from "node:crypto";

import { and, desc, eq } from "drizzle-orm";

import { db } from "@/db";
import { mcpToken, tenantCredentials } from "@/db/schema";
import { decryptSecret } from "@/lib/crypto";

export const MCP_SCOPES = ["read", "write"] as const;
export type McpScope = (typeof MCP_SCOPES)[number];

export type CreatedMcpToken = {
  id: string;
  name: string;
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

export async function createMcpToken(input: {
  organizationId: string;
  tenantId: string;
  userId: string;
  name: string;
  scopes: string[];
}): Promise<CreatedMcpToken> {
  const credential = await db
    .select()
    .from(tenantCredentials)
    .where(and(eq(tenantCredentials.organizationId, input.organizationId), eq(tenantCredentials.tenantId, input.tenantId)))
    .limit(1);
  if (!credential[0]) {
    throw new Error("TENANT_CREDENTIAL_NOT_FOUND");
  }

  const token = `mcp_${randomBytes(32).toString("base64url")}`;
  const scopes = normalizeScopes(input.scopes);
  const createdAt = new Date();
  const id = randomUUID();
  await db.insert(mcpToken).values({
    id,
    organizationId: input.organizationId,
    tenantId: input.tenantId,
    name: input.name.trim().slice(0, 80) || "Agente",
    tokenHash: hashToken(token),
    scopes: JSON.stringify(scopes),
    createdBy: input.userId,
    createdAt,
  });
  return { id, name: input.name.trim().slice(0, 80) || "Agente", scopes, token, createdAt };
}

export async function listMcpTokens(organizationId: string) {
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
    .where(eq(mcpToken.organizationId, organizationId))
    .orderBy(desc(mcpToken.createdAt));
}

export async function revokeMcpToken(organizationId: string, id: string): Promise<boolean> {
  const result = await db
    .update(mcpToken)
    .set({ revokedAt: new Date() })
    .where(and(eq(mcpToken.organizationId, organizationId), eq(mcpToken.id, id)))
    .returning({ id: mcpToken.id });
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

  const credential = await db
    .select()
    .from(tenantCredentials)
    .where(and(eq(tenantCredentials.organizationId, found.organizationId), eq(tenantCredentials.tenantId, found.tenantId)))
    .limit(1);
  if (!credential[0]) {
    return null;
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
    apiToken: decryptSecret(credential[0].encryptedToken),
    scopes,
    clientId: found.name,
    subject: found.tenantId,
  };
}
