import { adminToken, bridgeClient, bridgeConfigured, TENANT_ID } from "@/lib/bridge";
import type { components } from "@/lib/api-schema";

export type Schema = components["schemas"];
export type ApiResult<T> = { ok: true; data: T } | { ok: false; error: string; code?: string };

export const NOT_CONFIGURED_MESSAGE =
  "BRIDGE_API_TOKEN não está configurado no painel. Defina BRIDGE_URL e BRIDGE_API_TOKEN no ambiente para falar com o bridge.";

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

export async function loadStatus(): Promise<ApiResult<Schema["TenantView"]>> {
  if (!bridgeConfigured()) {
    return missing(NOT_CONFIGURED_MESSAGE, "PANEL_NOT_CONFIGURED");
  }
  return guard(async () => {
    const { data, error } = await bridgeClient().GET("/tenants/{tenant}/status", {
      params: { path: { tenant: TENANT_ID } },
      cache: "no-store",
    });
    if (error) {
      return failure(error);
    }
    const tenant = data?.data?.tenant;
    if (!tenant) {
      return missing("O bridge não retornou o status do tenant.", "EMPTY_RESPONSE");
    }
    return { ok: true, data: tenant };
  });
}

export async function loadStats(): Promise<ApiResult<Schema["TenantStats"]>> {
  if (!bridgeConfigured()) {
    return missing(NOT_CONFIGURED_MESSAGE, "PANEL_NOT_CONFIGURED");
  }
  return guard(async () => {
    const { data, error } = await bridgeClient().GET("/tenants/{tenant}/stats", {
      params: { path: { tenant: TENANT_ID } },
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
  if (!bridgeConfigured()) {
    return missing(NOT_CONFIGURED_MESSAGE, "PANEL_NOT_CONFIGURED");
  }
  return guard(async () => {
    const { data, error } = await bridgeClient().GET("/tenants/{tenant}/chats", {
      params: { path: { tenant: TENANT_ID }, query: { query: query || undefined, limit } },
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
  if (!bridgeConfigured()) {
    return missing(NOT_CONFIGURED_MESSAGE, "PANEL_NOT_CONFIGURED");
  }
  return guard(async () => {
    const { data, error } = await bridgeClient().GET("/tenants/{tenant}/chats/{jid}/messages", {
      params: { path: { tenant: TENANT_ID, jid }, query: { before: before || undefined, limit } },
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
  rule?: string;
  limit?: number;
}): Promise<ApiResult<EventPage>> {
  if (!bridgeConfigured()) {
    return missing(NOT_CONFIGURED_MESSAGE, "PANEL_NOT_CONFIGURED");
  }
  const { after, rule, limit = 100 } = options;
  return guard(async () => {
    const { data, error } = await bridgeClient().GET("/tenants/{tenant}/events", {
      params: {
        path: { tenant: TENANT_ID },
        query: { after: after || undefined, rule: rule || undefined, limit },
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

export async function loadRules(): Promise<ApiResult<Schema["EventRule"][]>> {
  if (!bridgeConfigured()) {
    return missing(NOT_CONFIGURED_MESSAGE, "PANEL_NOT_CONFIGURED");
  }
  return guard(async () => {
    const { data, error } = await bridgeClient().GET("/tenants/{tenant}/rules", {
      params: { path: { tenant: TENANT_ID } },
      cache: "no-store",
    });
    if (error) {
      return failure(error);
    }
    return { ok: true, data: data?.data?.rules ?? [] };
  });
}

export async function loadWebhooks(): Promise<ApiResult<Schema["WebhookView"][]>> {
  if (!bridgeConfigured()) {
    return missing(NOT_CONFIGURED_MESSAGE, "PANEL_NOT_CONFIGURED");
  }
  return guard(async () => {
    const { data, error } = await bridgeClient().GET("/tenants/{tenant}/webhooks", {
      params: { path: { tenant: TENANT_ID } },
      cache: "no-store",
    });
    if (error) {
      return failure(error);
    }
    return { ok: true, data: data?.data?.webhooks ?? [] };
  });
}

export async function loadKeys(): Promise<ApiResult<Schema["TenantKeyView"][]>> {
  if (!bridgeConfigured()) {
    return missing(NOT_CONFIGURED_MESSAGE, "PANEL_NOT_CONFIGURED");
  }
  return guard(async () => {
    const { data, error } = await bridgeClient().GET("/tenants/{tenant}/keys", {
      params: { path: { tenant: TENANT_ID } },
      cache: "no-store",
    });
    if (error) {
      return failure(error);
    }
    return { ok: true, data: data?.data?.keys ?? [] };
  });
}

export async function loadDeliveries(webhookId: string, limit = 25): Promise<ApiResult<Schema["DeliveryView"][]>> {
  if (!bridgeConfigured()) {
    return missing(NOT_CONFIGURED_MESSAGE, "PANEL_NOT_CONFIGURED");
  }
  return guard(async () => {
    const { data, error } = await bridgeClient().GET("/tenants/{tenant}/webhooks/{webhookId}/deliveries", {
      params: { path: { tenant: TENANT_ID, webhookId }, query: { limit } },
      cache: "no-store",
    });
    if (error) {
      return failure(error);
    }
    return { ok: true, data: data?.data?.deliveries ?? [] };
  });
}

export async function loadTenants(): Promise<ApiResult<Schema["TenantView"][]>> {
  const token = adminToken();
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
