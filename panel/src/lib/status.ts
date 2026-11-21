export type TenantStatusValue = "disconnected" | "pairing" | "connected" | "logged_out";

export const STATUS_LABELS: Record<TenantStatusValue, string> = {
  connected: "conectado",
  pairing: "pareando",
  disconnected: "desconectado",
  logged_out: "sessão encerrada",
};

export const STATUS_DOT_CLASSES: Record<TenantStatusValue, string> = {
  connected: "bg-emerald-500",
  pairing: "bg-highlight",
  disconnected: "bg-zinc-400",
  logged_out: "bg-red-500",
};

export const STATUS_BADGE_VARIANTS: Record<
  TenantStatusValue,
  "default" | "secondary" | "outline" | "destructive"
> = {
  connected: "default",
  pairing: "secondary",
  disconnected: "outline",
  logged_out: "destructive",
};

export function isTenantStatus(value: unknown): value is TenantStatusValue {
  return value === "disconnected" || value === "pairing" || value === "connected" || value === "logged_out";
}
