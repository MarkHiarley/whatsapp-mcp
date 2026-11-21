"use client";

import { useRouter } from "next/navigation";
import { useState, type FormEvent } from "react";
import { Send } from "lucide-react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { bridgePost } from "@/lib/client-api";

export function SendMessageForm({
  tenantId,
  jid,
  disabled,
}: {
  tenantId: string;
  jid: string;
  disabled: boolean;
}) {
  const router = useRouter();
  const [message, setMessage] = useState("");
  const [sending, setSending] = useState(false);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const text = message.trim();
    if (!text || sending) {
      return;
    }
    setSending(true);
    const result = await bridgePost<{ message: string }>(`/tenants/${tenantId}/send`, {
      recipient: jid,
      message: text,
    });
    setSending(false);
    if (!result.ok) {
      toast.error(result.error);
      return;
    }
    setMessage("");
    toast.success("Mensagem enviada.");
    router.refresh();
  }

  return (
    <form onSubmit={submit} className="space-y-2">
      <Textarea
        value={message}
        onChange={(event) => setMessage(event.target.value)}
        placeholder={disabled ? "Conecte o WhatsApp para enviar mensagens" : "Escreva uma mensagem…"}
        disabled={disabled || sending}
        rows={2}
        className="min-h-16 resize-none"
        onKeyDown={(event) => {
          if (event.key === "Enter" && !event.shiftKey) {
            event.preventDefault();
            event.currentTarget.form?.requestSubmit();
          }
        }}
      />
      <div className="flex items-center justify-between gap-2">
        <p className="text-xs text-muted-foreground">
          {disabled ? "Envio disponível apenas com a sessão conectada." : "Enter envia · Shift+Enter quebra linha"}
        </p>
        <Button type="submit" size="sm" disabled={disabled || sending || message.trim().length === 0}>
          <Send aria-hidden />
          {sending ? "Enviando…" : "Enviar"}
        </Button>
      </div>
    </form>
  );
}
