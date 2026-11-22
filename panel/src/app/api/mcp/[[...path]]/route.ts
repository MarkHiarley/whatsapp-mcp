import { NextResponse } from "next/server";


const HOP_BY_HOP = new Set([
  "connection",
  "keep-alive",
  "proxy-authenticate",
  "proxy-authorization",
  "te",
  "trailer",
  "transfer-encoding",
  "upgrade",
  "host",
  "content-length",
  "content-encoding",
]);

async function forward(request: Request, path: string[] | undefined) {
  const internal = process.env.MCP_INTERNAL_URL?.trim().replace(/\/+$/, "");
  if (!internal) {
    return NextResponse.json({ error: "MCP_NOT_CONFIGURED" }, { status: 503 });
  }
  const suffix = path && path.length > 0 ? `/${path.join("/")}` : "";
  const target = `${internal}/api/mcp${suffix}`;
  const headers = new Headers();
  for (const [key, value] of request.headers) {
    if (!HOP_BY_HOP.has(key.toLowerCase())) {
      headers.set(key, value);
    }
  }
  const method = request.method.toUpperCase();
  const body = method === "GET" || method === "HEAD" ? undefined : await request.arrayBuffer();
  let upstream: Response;
  try {
    upstream = await fetch(target, { method, headers, body, redirect: "manual" });
  } catch {
    return NextResponse.json({ error: "MCP_UNAVAILABLE" }, { status: 502 });
  }

  const responseHeaders = new Headers();
  for (const [key, value] of upstream.headers) {
    if (!HOP_BY_HOP.has(key.toLowerCase())) {
      responseHeaders.set(key, value);
    }
  }
  responseHeaders.set("Cache-Control", "no-store");
  return new Response(upstream.body, { status: upstream.status, headers: responseHeaders });
}

export async function GET(request: Request, { params }: { params: Promise<{ path?: string[] }> }) {
  return forward(request, (await params).path);
}
export async function POST(request: Request, { params }: { params: Promise<{ path?: string[] }> }) {
  return forward(request, (await params).path);
}
export async function DELETE(request: Request, { params }: { params: Promise<{ path?: string[] }> }) {
  return forward(request, (await params).path);
}
