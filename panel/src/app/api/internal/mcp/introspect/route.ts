import { timingSafeEqual } from "node:crypto";

import { NextResponse } from "next/server";

import { introspectMcpToken } from "@/lib/mcp-tokens";


function safeEqual(a: string, b: string): boolean {
  const left = Buffer.from(a);
  const right = Buffer.from(b);
  return left.length === right.length && timingSafeEqual(left, right);
}

export async function POST(request: Request) {
  const secret = process.env.MCP_INTROSPECT_SECRET;
  if (!secret) {
    return NextResponse.json({ error: "NOT_CONFIGURED" }, { status: 503 });
  }
  const authorization = request.headers.get("authorization") ?? "";
  const provided = authorization.startsWith("Bearer ") ? authorization.slice(7) : "";
  if (!provided || !safeEqual(provided, secret)) {
    return NextResponse.json({ error: "UNAUTHORIZED" }, { status: 401 });
  }

  let body: { token?: unknown };
  try {
    body = await request.json();
  } catch {
    return NextResponse.json({ error: "INVALID_REQUEST" }, { status: 400 });
  }
  if (typeof body.token !== "string" || !body.token.startsWith("mcp_")) {
    return NextResponse.json({ error: "INVALID_REQUEST" }, { status: 400 });
  }

  const found = await introspectMcpToken(body.token);
  if (!found) {
    return NextResponse.json({ error: "UNAUTHORIZED" }, { status: 401 });
  }
  return NextResponse.json(
    {
      tenant: found.tenant,
      api_token: found.apiToken,
      scopes: found.scopes,
      client_id: found.clientId,
      subject: found.subject,
      token_type: "Bearer",
    },
    { headers: { "Cache-Control": "no-store" } },
  );
}
