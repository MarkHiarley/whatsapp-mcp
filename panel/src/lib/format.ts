const dateTimeFormatter = new Intl.DateTimeFormat("pt-BR", {
  dateStyle: "short",
  timeStyle: "short",
});

const timeFormatter = new Intl.DateTimeFormat("pt-BR", {
  hour: "2-digit",
  minute: "2-digit",
});

const numberFormatter = new Intl.NumberFormat("pt-BR");

function toDate(value?: string | null): Date | null {
  if (!value) {
    return null;
  }
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? null : date;
}

export function formatDateTime(value?: string | null): string {
  const date = toDate(value);
  return date ? dateTimeFormatter.format(date) : "—";
}

export function formatTime(value?: string | null): string {
  const date = toDate(value);
  return date ? timeFormatter.format(date) : "—";
}

export function formatNumber(value?: number | null): string {
  return typeof value === "number" ? numberFormatter.format(value) : "—";
}

const relativeFormatter = new Intl.RelativeTimeFormat("pt-BR", { numeric: "auto" });

export function formatRelative(value?: string | null): string {
  const date = toDate(value);
  if (!date) {
    return "—";
  }
  const diffSeconds = Math.round((date.getTime() - Date.now()) / 1000);
  const absolute = Math.abs(diffSeconds);
  if (absolute < 60) {
    return relativeFormatter.format(diffSeconds, "second");
  }
  if (absolute < 3600) {
    return relativeFormatter.format(Math.round(diffSeconds / 60), "minute");
  }
  if (absolute < 86_400) {
    return relativeFormatter.format(Math.round(diffSeconds / 3600), "hour");
  }
  if (absolute < 2_592_000) {
    return relativeFormatter.format(Math.round(diffSeconds / 86_400), "day");
  }
  return formatDateTime(value);
}

export function phoneFromJid(jid: string): string {
  const [user] = jid.split("@");
  return user || jid;
}

export function initials(name?: string | null): string {
  const source = (name ?? "").trim();
  if (!source) {
    return "?";
  }
  const parts = source.split(/\s+/).slice(0, 2);
  const result = parts.map((part) => part.charAt(0).toUpperCase()).join("");
  return result || "?";
}

export function truncate(value: string, max = 120): string {
  if (value.length <= max) {
    return value;
  }
  return `${value.slice(0, max - 1)}…`;
}

export function mediaLabel(mediaType?: string | null, filename?: string | null): string {
  if (filename) {
    return filename;
  }
  if (!mediaType) {
    return "arquivo";
  }
  const labels: Record<string, string> = {
    image: "imagem",
    video: "vídeo",
    audio: "áudio",
    document: "documento",
    sticker: "figurinha",
  };
  return labels[mediaType] ?? mediaType;
}
