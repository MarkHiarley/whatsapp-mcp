import { NextResponse } from "next/server";

import { createMcpToken, listMcpTokens, MCP_SCOPES } from "@/lib/mcp-tokens";
import { getTenantContext } from "@/lib/session";


function canManage(role: string): boolean {
  return role === "admin" || role === "owner";
}

export async function GET() {
  const context = await getTenantContext();
  if (!context) {
    return NextResponse.json({ error: "UNAUTHORIZED" }, { status: 401 });
  }
  if (!canManage(context.role)) {
    return NextResponse.json({ error: "FORBIDDEN" }, { status: 403 });
  }
  const tokens = await listMcpTokens(context.organizationId, context.tenantId);
  return NextResponse.json({ tokens });
}

export async function POST(request: Request) {
  const context = await getTenantContext();
  if (!context) {
    return NextResponse.json({ error: "UNAUTHORIZED" }, { status: 401 });
  }
  if (!canManage(context.role)) {
    return NextResponse.json({ error: "FORBIDDEN" }, { status: 403 });
  }

  let body: { name?: unknown; scopes?: unknown };
  try {
    body = await request.json();
  } catch {
    return NextResponse.json({ error: "INVALID_REQUEST" }, { status: 400 });
  }
  const name = typeof body.name === "string" ? body.name.trim() : "";
  if (!name || name.length > 80) {
    return NextResponse.json({ error: "INVALID_NAME" }, { status: 400 });
  }
  const scopes = Array.isArray(body.scopes)
    ? body.scopes.filter((scope): scope is string => typeof scope === "string" && MCP_SCOPES.includes(scope as never))
    : ["read"];
  if (scopes.some((scope) => !MCP_SCOPES.includes(scope as never))) {
    return NextResponse.json({ error: "INVALID_SCOPES" }, { status: 400 });
  }

  try {
    const created = await createMcpToken({
      organizationId: context.organizationId,
      tenantId: context.tenantId,
      userId: context.userId,
      name,
      scopes,
      isAdmin: context.isAdmin,
    });
    return NextResponse.json(
      {
        id: created.id,
        name: created.name,
        tenantId: created.tenantId,
        scopes: created.scopes,
        token: created.token,
        createdAt: created.createdAt.toISOString(),
      },
      { headers: { "Cache-Control": "no-store" } },
    );
  } catch (error) {
    const code = error instanceof Error ? error.message : "INTERNAL_ERROR";
    const status =
      code === "TENANT_CREDENTIAL_NOT_FOUND" || code === "TENANT_NOT_FOUND" || code === "TENANT_KEY_INVALID" || code === "TENANT_FORBIDDEN"
        ? 400
        : code === "BRIDGE_NOT_CONFIGURED" || code === "BRIDGE_ADMIN_NOT_CONFIGURED" || code === "BRIDGE_UNAVAILABLE" || code === "BRIDGE_KEY_CREATE_FAILED"
          ? 503
          : 500;
    return NextResponse.json({ error: code }, { status });
  }
}
