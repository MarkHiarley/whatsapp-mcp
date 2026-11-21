import createClient from "openapi-fetch";
import type { paths } from "@/lib/api-schema";

export const BRIDGE_URL = (process.env.BRIDGE_URL ?? "http://127.0.0.1:8080/api/v1").replace(/\/$/, "");
export const TENANT_ID = process.env.BRIDGE_TENANT_ID ?? "default";

export function tenantToken(): string {
  const token = process.env.BRIDGE_API_TOKEN;
  if (!token) {
    throw new Error("BRIDGE_API_TOKEN is not configured");
  }
  return token;
}

export function adminToken(): string {
  return process.env.BRIDGE_ADMIN_TOKEN ?? "";
}

export function bridgeClient(token: string = tenantToken()) {
  return createClient<paths>({
    baseUrl: BRIDGE_URL,
    headers: { Authorization: `Bearer ${token}` },
  });
}

export function bridgeConfigured(): boolean {
  return Boolean(process.env.BRIDGE_API_TOKEN);
}
