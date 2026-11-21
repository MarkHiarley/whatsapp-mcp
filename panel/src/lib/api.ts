import type { Client } from "openapi-fetch";

import type { components, paths } from "@/lib/api-schema";
import { adminBridgeToken, bridgeClient } from "@/lib/bridge";
import { getTenantContext } from "@/lib/session";

export type Schema = components["schemas"];
export type ApiResult<T> = { ok: true; data: T } | { ok: false; error: string; code?: string };

export const NOT_CONNECTED_MESSAGE =
  "Nenhum tenant está vinculado a esta conta. Conecte uma key do bridge para continuar.";

type BridgeClient = Client<paths>;
type EnvelopeLike = { error?: { code?: string; message?: string } | null };

function failure(error: unknown): { ok: false; error: string; code?: string } {
  const payload = error as EnvelopeLike | undefined;
  return {
    ok: false,
    error: payload?.error?.message ?? "O bridge retornou um erro sem mensagem.",
    code: payload?.error?.code,
  };
}

async function guard<T>(run: () => Promise<ApiResult<T>>): Promise<ApiResult<T>> {
  try {
    return await run();
  } catch (error) {
    const message = error instanceof Error ? error.message : "falha de rede";
    return { ok: false, error: `Não foi possível falar com o bridge: ${message}` };
  }
}

function missing<T>(message: string, code?: string): ApiResult<T> {
  return { ok: false, error: message, code };
}

async function withTenant<T>(
  run: (client: BridgeClient, tenant: string) => Promise<ApiResult<T>>,
): Promise<ApiResult<T>> {
  const context = await getTenantContext();
  if (!context) {
    return missing(NOT_CONNECTED_MESSAGE, "TENANT_NOT_CONNECTED");
  }
  return guard(() => run(bridgeClient(context.token), context.tenantId));
}

export async function loadStatus(): Promise<ApiResult<Schema["TenantView"]>> {
  return withTenant(async (client, tenant) => {
    const { data, error } = await client.GET("/tenants/{tenant}/status", {
      params: { path: { tenant } },
      cache: "no-store",
    });
    if (error) {
      return failure(error);
    }
    const view = data?.data?.tenant;
    if (!view) {
      return missing("O bridge não retornou o status do tenant.", "EMPTY_RESPONSE");
    }
    return { ok: true, data: view };
  });
}

export async function loadStats(): Promise<ApiResult<Schema["TenantStats"]>> {
  return withTenant(async (client, tenant) => {
    const { data, error } = await client.GET("/tenants/{tenant}/stats", {
      params: { path: { tenant } },
      cache: "no-store",
    });
    if (error) {
      return failure(error);
    }
    const stats = data?.data?.stats;
    if (!stats) {
      return missing("O bridge não retornou as estatísticas do tenant.", "EMPTY_RESPONSE");
    }
    return { ok: true, data: stats };
  });
}

export async function loadChats(query?: string, limit = 50): Promise<ApiResult<Schema["ChatListItem"][]>> {
  return withTenant(async (client, tenant) => {
    const { data, error } = await client.GET("/tenants/{tenant}/chats", {
      params: { path: { tenant }, query: { query: query || undefined, limit } },
      cache: "no-store",
    });
    if (error) {
      return failure(error);
    }
    return { ok: true, data: data?.data?.chats ?? [] };
  });
}

export type MessagePage = { messages: Schema["MessageListItem"][]; hasMore: boolean };

export async function loadMessages(jid: string, before?: string, limit = 50): Promise<ApiResult<MessagePage>> {
  return withTenant(async (client, tenant) => {
    const { data, error } = await client.GET("/tenants/{tenant}/chats/{jid}/messages", {
      params: { path: { tenant, jid }, query: { before: before || undefined, limit } },
      cache: "no-store",
    });
    if (error) {
      return failure(error);
    }
    return {
      ok: true,
      data: { messages: data?.data?.messages ?? [], hasMore: data?.data?.has_more ?? false },
    };
  });
}

export type EventPage = { events: Schema["Event"][]; nextAfter: number };

export async function loadEvents(options: {
  after?: number;
  limit?: number;
}): Promise<ApiResult<EventPage>> {
  const { after, limit = 100 } = options;
  return withTenant(async (client, tenant) => {
    const { data, error } = await client.GET("/tenants/{tenant}/events", {
      params: {
        path: { tenant },
        query: { after: after || undefined, limit },
      },
      cache: "no-store",
    });
    if (error) {
      return failure(error);
    }
    return {
      ok: true,
      data: { events: data?.data?.events ?? [], nextAfter: data?.data?.next_after ?? after ?? 0 },
    };
  });
}


export async function loadWebhooks(): Promise<ApiResult<Schema["WebhookView"][]>> {
  return withTenant(async (client, tenant) => {
    const { data, error } = await client.GET("/tenants/{tenant}/webhooks", {
      params: { path: { tenant } },
      cache: "no-store",
    });
    if (error) {
      return failure(error);
    }
    return { ok: true, data: data?.data?.webhooks ?? [] };
  });
}

export async function loadKeys(): Promise<ApiResult<Schema["TenantKeyView"][]>> {
  return withTenant(async (client, tenant) => {
    const { data, error } = await client.GET("/tenants/{tenant}/keys", {
      params: { path: { tenant } },
      cache: "no-store",
    });
    if (error) {
      return failure(error);
    }
    return { ok: true, data: data?.data?.keys ?? [] };
  });
}

export async function loadDeliveries(webhookId: string, limit = 25): Promise<ApiResult<Schema["DeliveryView"][]>> {
  return withTenant(async (client, tenant) => {
    const { data, error } = await client.GET("/tenants/{tenant}/webhooks/{webhookId}/deliveries", {
      params: { path: { tenant, webhookId }, query: { limit } },
      cache: "no-store",
    });
    if (error) {
      return failure(error);
    }
    return { ok: true, data: data?.data?.deliveries ?? [] };
  });
}

export async function loadTenants(): Promise<ApiResult<Schema["TenantView"][]>> {
  const context = await getTenantContext();
  if (!context) {
    return missing(NOT_CONNECTED_MESSAGE, "TENANT_NOT_CONNECTED");
  }
  if (!context.isAdmin) {
    return missing("Apenas admin pode listar e provisionar tenants.", "FORBIDDEN");
  }
  const token = adminBridgeToken();
  if (!token) {
    return missing(
      "BRIDGE_ADMIN_TOKEN não está configurado no painel. Defina o token admin para listar e provisionar tenants.",
      "PANEL_ADMIN_NOT_CONFIGURED",
    );
  }
  return guard(async () => {
    const { data, error } = await bridgeClient(token).GET("/tenants", { cache: "no-store" });
    if (error) {
      return failure(error);
    }
    return { ok: true, data: data?.data?.tenants ?? [] };
  });
}
