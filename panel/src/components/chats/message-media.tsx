import { Download } from "lucide-react";

import type { Schema } from "@/lib/api";
import { TENANT_ID } from "@/lib/bridge";
import { mediaLabel } from "@/lib/format";

function mediaUrl(messageId: string, jid: string): string {
  const tenant = encodeURIComponent(TENANT_ID);
  return `/api/bridge/tenants/${tenant}/media/${encodeURIComponent(messageId)}?chat_jid=${encodeURIComponent(jid)}`;
}

export function MessageMedia({ message, jid }: { message: Schema["MessageListItem"]; jid: string }) {
  if (!message.media_type) {
    return null;
  }

  const src = mediaUrl(message.id, jid);
  const label = mediaLabel(message.media_type, message.filename);

  if (message.media_type === "image") {
    return (
      // eslint-disable-next-line @next/next/no-img-element -- bytes are streamed by the BFF, not a static asset
      <img
        src={src}
        alt={label}
        loading="lazy"
        className="mb-1 max-h-80 w-auto max-w-full rounded-lg border border-black/5"
      />
    );
  }

  if (message.media_type === "video") {
    return <video src={src} controls preload="metadata" className="mb-1 max-h-80 w-full rounded-lg" />;
  }

  if (message.media_type === "audio") {
    return <audio src={src} controls preload="metadata" className="mb-1 w-full min-w-56" />;
  }

  return (
    <a
      href={src}
      download={message.filename ?? undefined}
      className="mb-1 inline-flex max-w-full items-center gap-1.5 rounded-lg border px-2 py-1 text-xs underline-offset-2 hover:underline"
      target="_blank"
      rel="noreferrer"
    >
      <Download className="size-3.5 shrink-0" aria-hidden />
      <span className="truncate">{label}</span>
    </a>
  );
}
