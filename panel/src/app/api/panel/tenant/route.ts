import { cookies } from "next/headers";
import { NextResponse, type NextRequest } from "next/server";

import { BRIDGE_URL, bridgeHeaders } from "@/lib/bridge";
import { getSession, isPanelAdmin, TENANT_COOKIE } from "@/lib/session";

const TENANT_ID = /^[a-z0-9][a-z0-9-]{1,31}$/;

export async function POST(request: NextRequest) {
  const session = await getSession();
  if (!session) {
    return NextResponse.json({ ok: false, error: "unauthorized" }, { status: 401 });
  }
  if (!(await isPanelAdmin(session.user.email, session.user.id))) {
    return NextResponse.json({ ok: false, error: "admin only" }, { status: 403 });
  }

  const body = (await request.json().catch(() => null)) as { tenant?: unknown } | null;
  const tenant = typeof body?.tenant === "string" ? body.tenant.trim() : "";
  if (!TENANT_ID.test(tenant)) {
    return NextResponse.json({ ok: false, error: "invalid tenant" }, { status: 400 });
  }

  const adminToken = process.env.BRIDGE_ADMIN_TOKEN;
  if (!adminToken) {
    return NextResponse.json({ ok: false, error: "BRIDGE_ADMIN_TOKEN is not configured" }, { status: 503 });
  }

  const response = await fetch(`${BRIDGE_URL}/tenants/${encodeURIComponent(tenant)}/status`, {
    headers: bridgeHeaders(adminToken),
    cache: "no-store",
  });
  if (!response.ok) {
    return NextResponse.json({ ok: false, error: "tenant not found" }, { status: 404 });
  }

  const store = await cookies();
  store.set(TENANT_COOKIE, tenant, {
    httpOnly: true,
    sameSite: "lax",
    secure: process.env.NODE_ENV === "production",
    path: "/",
    maxAge: 60 * 60 * 24 * 30,
  });

  return NextResponse.json({ ok: true, tenant });
}
