import { NextResponse } from "next/server";

import { revokeMcpToken } from "@/lib/mcp-tokens";
import { getTenantContext } from "@/lib/session";


export async function DELETE(_: Request, { params }: { params: Promise<{ id: string }> }) {
  const context = await getTenantContext();
  if (!context) {
    return NextResponse.json({ error: "UNAUTHORIZED" }, { status: 401 });
  }
  if (context.role !== "admin" && context.role !== "owner") {
    return NextResponse.json({ error: "FORBIDDEN" }, { status: 403 });
  }
  const { id } = await params;
  const revoked = await revokeMcpToken(context.organizationId, context.tenantId, id);
  if (!revoked) {
    return NextResponse.json({ error: "NOT_FOUND" }, { status: 404 });
  }
  return NextResponse.json({ ok: true });
}
