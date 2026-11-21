export type ClientResult<T> = { ok: true; data: T } | { ok: false; error: string; code?: string };

type EnvelopeBody = {
  success?: boolean;
  data?: unknown;
  error?: { code?: string; message?: string } | null;
};

/**
 * Calls the panel BFF. The bridge token never reaches the browser: the route
 * handler attaches it to the upstream request.
 */
export async function bridgeRequest<T = unknown>(path: string, init?: RequestInit): Promise<ClientResult<T>> {
  const hasBody = init?.body !== undefined && init.body !== null;
  const headers = new Headers(init?.headers);
  if (hasBody && !headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json");
  }

  try {
    const response = await fetch(`/api/bridge/${path.replace(/^\/+/, "")}`, { ...init, headers, cache: "no-store" });
    let body: EnvelopeBody | null = null;
    try {
      body = (await response.json()) as EnvelopeBody;
    } catch {
      body = null;
    }

    if (!response.ok || body?.success === false) {
      return {
        ok: false,
        error: body?.error?.message ?? `A requisição falhou (HTTP ${response.status}).`,
        code: body?.error?.code,
      };
    }

    return { ok: true, data: (body?.data ?? null) as T };
  } catch (error) {
    return { ok: false, error: error instanceof Error ? error.message : "Falha de rede." };
  }
}

export function bridgeGet<T = unknown>(path: string): Promise<ClientResult<T>> {
  return bridgeRequest<T>(path, { method: "GET" });
}

export function bridgePost<T = unknown>(path: string, payload?: unknown): Promise<ClientResult<T>> {
  return bridgeRequest<T>(path, {
    method: "POST",
    body: payload === undefined ? undefined : JSON.stringify(payload),
  });
}

export function bridgePatch<T = unknown>(path: string, payload: unknown): Promise<ClientResult<T>> {
  return bridgeRequest<T>(path, { method: "PATCH", body: JSON.stringify(payload) });
}

export function bridgeDelete<T = unknown>(path: string): Promise<ClientResult<T>> {
  return bridgeRequest<T>(path, { method: "DELETE" });
}
