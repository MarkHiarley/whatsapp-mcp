import createClient from "openapi-fetch";

import type { paths } from "@/lib/api-schema";

export const BRIDGE_URL = (process.env.BRIDGE_URL ?? "http://127.0.0.1:8080/api/v1").replace(/\/$/, "");

export function adminBridgeToken(): string | null {
  return process.env.BRIDGE_ADMIN_TOKEN ?? null;
}

export function bridgeClient(token: string) {
  return createClient<paths>({
    baseUrl: BRIDGE_URL,
    headers: { Authorization: `Bearer ${token}` },
  });
}
