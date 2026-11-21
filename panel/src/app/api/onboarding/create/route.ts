import { NextResponse, type NextRequest } from "next/server";

import { createAndLinkTenant, isValidTenantId } from "@/lib/onboarding";
import { getSession, isPanelAdmin } from "@/lib/session";

type Body = { tenantId?: unknown; label?: unknown };

export async function POST(request: NextRequest) {
  const session = await getSession();
  if (!session) {
    return NextResponse.json({ ok: false, error: "unauthorized" }, { status: 401 });
  }
  if (!(await isPanelAdmin(session.user.email, session.user.id))) {
    return NextResponse.json({ ok: false, error: "Apenas admin pode criar tenants." }, { status: 403 });
  }
  const body = (await request.json().catch(() => null)) as Body | null;
  const tenantId = typeof body?.tenantId === "string" ? body.tenantId.trim() : "";
  const label = typeof body?.label === "string" && body.label.trim() ? body.label.trim() : tenantId;

  if (!isValidTenantId(tenantId)) {
    return NextResponse.json({ ok: false, error: "Tenant id inválido." }, { status: 400 });
  }
  if (!label) {
    return NextResponse.json({ ok: false, error: "Informe um nome." }, { status: 400 });
  }

  const result = await createAndLinkTenant(session.user.id, tenantId, label);
  if (!result.ok) {
    return NextResponse.json({ ok: false, error: result.error }, { status: 400 });
  }
  return NextResponse.json({ ok: true, tenantId: result.tenantId });
}
