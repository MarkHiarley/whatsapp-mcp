import type { NextRequest } from "next/server";

const BASE = (process.env.BRIDGE_URL ?? "http://127.0.0.1:8080/api/v1").replace(/\/$/, "");

function resolveToken(segments: string[], method: string): string | null {
  const isTenantCollection = segments.length === 1 && segments[0] === "tenants";
  const isTenantItem = segments.length === 2 && segments[0] === "tenants";
  if (segments[0] !== "tenants") {
    return null;
  }
  if (isTenantCollection || (isTenantItem && method === "DELETE")) {
    return process.env.BRIDGE_ADMIN_TOKEN || null;
  }
  return process.env.BRIDGE_API_TOKEN || null;
}

async function proxy(request: NextRequest, context: { params: Promise<{ path: string[] }> }) {
  const { path } = await context.params;
  const token = resolveToken(path, request.method);
  if (!token) {
    return Response.json(
      { success: false, data: null, error: { code: "PANEL_NOT_CONFIGURED", message: "Bridge token is not configured" } },
      { status: 503 },
    );
  }

  const url = new URL(request.url);
  const target = `${BASE}/${path.map(encodeURIComponent).join("/")}${url.search}`;
  const headers = new Headers({ Authorization: `Bearer ${token}` });
  const contentType = request.headers.get("content-type");
  if (contentType) {
    headers.set("Content-Type", contentType);
  }
  const accept = request.headers.get("accept");
  if (accept) {
    headers.set("Accept", accept);
  }

  const hasBody = request.method !== "GET" && request.method !== "HEAD";
  const upstream = await fetch(target, {
    method: request.method,
    headers,
    body: hasBody ? await request.text() : undefined,
    cache: "no-store",
    redirect: "manual",
  });

  const responseHeaders = new Headers();
  for (const key of ["content-type", "content-disposition", "cache-control", "deprecation"]) {
    const value = upstream.headers.get(key);
    if (value) {
      responseHeaders.set(key, value);
    }
  }
  if (responseHeaders.get("content-type")?.includes("text/event-stream")) {
    responseHeaders.set("Cache-Control", "no-cache, no-transform");
    responseHeaders.set("Connection", "keep-alive");
  }

  return new Response(upstream.body, {
    status: upstream.status,
    headers: responseHeaders,
  });
}

export const GET = proxy;
export const POST = proxy;
export const PATCH = proxy;
export const DELETE = proxy;
export const HEAD = proxy;
