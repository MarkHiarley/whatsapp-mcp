import { NextResponse, type NextRequest } from "next/server";

import { connectExistingTenant, isValidTenantId } from "@/lib/onboarding";
import { getSession } from "@/lib/session";

type Body = { tenantId?: unknown; key?: unknown; label?: unknown };

export async function POST(request: NextRequest) {
  const session = await getSession();
  if (!session) {
    return NextResponse.json({ ok: false, error: "unauthorized" }, { status: 401 });
  }
  const body = (await request.json().catch(() => null)) as Body | null;
  const tenantId = typeof body?.tenantId === "string" ? body.tenantId.trim() : "";
  const key = typeof body?.key === "string" ? body.key.trim() : "";
  const label = typeof body?.label === "string" && body.label.trim() ? body.label.trim() : tenantId;

  if (!isValidTenantId(tenantId)) {
    return NextResponse.json({ ok: false, error: "Tenant id inválido." }, { status: 400 });
  }
  if (!key) {
    return NextResponse.json({ ok: false, error: "Informe a key do tenant." }, { status: 400 });
  }

  const result = await connectExistingTenant(session.user.id, tenantId, key, label);
  if (!result.ok) {
    return NextResponse.json({ ok: false, error: result.error }, { status: 400 });
  }
  return NextResponse.json({ ok: true, tenantId: result.tenantId });
}
