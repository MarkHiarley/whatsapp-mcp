import type { NextRequest } from "next/server";

import { getTenantContext } from "@/lib/session";

const BASE = (process.env.BRIDGE_URL ?? "http://127.0.0.1:8080/api/v1").replace(/\/$/, "");

function isAdminRoute(segments: string[], method: string): boolean {
  if (segments[0] !== "tenants") {
    return false;
  }
  if (segments.length === 1) {
    return true;
  }
  return segments.length === 2 && method === "DELETE";
}

function error(status: number, code: string, message: string): Response {
  return Response.json({ success: false, data: null, error: { code, message } }, { status });
}

async function proxy(request: NextRequest, context: { params: Promise<{ path: string[] }> }) {
  const { path } = await context.params;
  if (path[0] !== "tenants") {
    return error(404, "NOT_FOUND", "Unknown panel route");
  }

  const session = await getTenantContext();
  if (!session) {
    return error(401, "UNAUTHORIZED", "Sign in to use the panel");
  }

  const adminRoute = isAdminRoute(path, request.method);
  const sensitiveWrite =
    ["keys", "webhooks", "rules"].some((segment) => path.includes(segment)) &&
    ["POST", "PATCH", "DELETE"].includes(request.method);
  if (!session.isAdmin && sensitiveWrite && session.role !== "owner") {
    return error(403, "FORBIDDEN", "Only the tenant owner can change keys, rules or webhooks");
  }

  let token: string | null;
  if (adminRoute) {
    if (!session.isAdmin) {
      return error(403, "FORBIDDEN", "Admin access required");
    }
    token = process.env.BRIDGE_ADMIN_TOKEN ?? null;
    if (!token) {
      return error(503, "ADMIN_DISABLED", "BRIDGE_ADMIN_TOKEN is not configured");
    }
  } else {
    if (path[1] && !session.isAdmin && path[1] !== session.tenantId) {
      return error(403, "FORBIDDEN", "This key belongs to another tenant");
    }
    token = session.token;
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
