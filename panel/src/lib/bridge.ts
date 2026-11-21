import createClient from "openapi-fetch";

import type { paths } from "@/lib/api-schema";

export const BRIDGE_URL = (process.env.BRIDGE_URL ?? "http://127.0.0.1:8080/api/v1").replace(/\/$/, "");

export function adminBridgeToken(): string | null {
  return process.env.BRIDGE_ADMIN_TOKEN ?? null;
}

export function bridgeHeaders(token: string): Record<string, string> {
  const headers: Record<string, string> = { Authorization: `Bearer ${token}` };
  const clientId = process.env.BRIDGE_CF_ACCESS_CLIENT_ID;
  const clientSecret = process.env.BRIDGE_CF_ACCESS_CLIENT_SECRET;
  if (clientId && clientSecret) {
    headers["CF-Access-Client-Id"] = clientId;
    headers["CF-Access-Client-Secret"] = clientSecret;
  }
  return headers;
}

export function bridgeClient(token: string) {
  return createClient<paths>({
    baseUrl: BRIDGE_URL,
    headers: bridgeHeaders(token),
  });
}
