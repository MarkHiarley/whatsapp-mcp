"use client";

import { useRouter } from "next/navigation";
import { useState, type FormEvent } from "react";
import { Plus } from "lucide-react";
import { toast } from "sonner";

import { SecretDialog } from "@/components/secret-dialog";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import type { Schema } from "@/lib/api";
import { bridgePost } from "@/lib/client-api";

function linesToArray(value: string): string[] {
  return value
    .split("\n")
    .map((line) => line.trim())
    .filter((line) => line.length > 0);
}

function parseHeaderLines(value: string): { headers: Record<string, string>; error?: string } {
  const headers: Record<string, string> = {};
  for (const line of linesToArray(value)) {
    const separator = line.indexOf(":");
    if (separator <= 0) {
      return { headers: {}, error: `Header inválido: “${line}”. Use Nome: valor.` };
    }
    const name = line.slice(0, separator).trim();
    const headerValue = line.slice(separator + 1).trim();
    if (!name || !headerValue) {
      return { headers: {}, error: `Header inválido: “${line}”. Use Nome: valor.` };
    }
    headers[name] = headerValue;
  }
  return { headers };
}

type ChatType = NonNullable<Schema["RuleMatch"]["chat_types"]>[number];
type MessageType = NonNullable<Schema["RuleMatch"]["message_types"]>[number];

const CHAT_TYPES: { value: ChatType; label: string }[] = [
  { value: "direct", label: "privados" },
  { value: "group", label: "grupos" },
  { value: "status", label: "status" },
  { value: "newsletter", label: "canais" },
  { value: "broadcast", label: "broadcast" },
  { value: "other", label: "outros" },
];

const MESSAGE_TYPES: MessageType[] = [
  "text", "image", "video", "audio", "document", "sticker", "location", "contact",
  "poll", "poll_update", "reaction", "protocol", "interactive", "album", "event", "other",
];

function toggleValue<T extends string>(current: T[], value: T): T[] {
  return current.includes(value) ? current.filter((item) => item !== value) : [...current, value];
}

export function NewWebhookDialog({ tenantId }: { tenantId: string }) {
  const router = useRouter();
  const [open, setOpen] = useState(false);
  const [saving, setSaving] = useState(false);
  const [url, setUrl] = useState("");
  const [headerText, setHeaderText] = useState("");
  const [chatJids, setChatJids] = useState("");
  const [senderJids, setSenderJids] = useState("");
  const [textContains, setTextContains] = useState("");
  const [chatTypes, setChatTypes] = useState<ChatType[]>(["direct"]);
  const [messageTypes, setMessageTypes] = useState<MessageType[]>(["text", "image", "video", "audio", "document"]);
  const [inbound, setInbound] = useState(true);
  const [mention, setMention] = useState(false);
  const [reply, setReply] = useState(false);
  const [secret, setSecret] = useState<string | null>(null);
  const [secretOpen, setSecretOpen] = useState(false);

  function buildQuickMatch(): Schema["RuleMatch"] {
    const match: Schema["RuleMatch"] = {};
    const chats = linesToArray(chatJids);
    if (chats.length > 0) {
      match.chat_jids = chats;
    }
    const senders = linesToArray(senderJids);
    if (senders.length > 0) {
      match.sender_jids = senders;
    }
    const contains = linesToArray(textContains);
    if (contains.length > 0) {
      match.text_contains = contains;
    }
    if (chatTypes.length > 0) {
      match.chat_types = chatTypes;
    }
    if (messageTypes.length > 0) {
      match.message_types = messageTypes;
    }
    if (inbound) {
      match.is_from_me = false;
    }
    if (mention) {
      match.mentioned_me = true;
    }
    if (reply) {
      match.is_reply_to_me = true;
    }
    return match;
  }

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!url.trim()) {
      toast.error("Informe a URL do webhook.");
      return;
    }
    const parsedHeaders = parseHeaderLines(headerText);
    if (parsedHeaders.error) {
      toast.error(parsedHeaders.error);
      return;
    }
    const customHeaders = parsedHeaders.headers;

    setSaving(true);
    let host = "webhook";
    try {
      host = new URL(url).host;
    } catch {
      // URL validation is handled by the input and the bridge.
    }
    const created = await bridgePost<{ rule: Schema["EventRule"] }>(`/tenants/${tenantId}/rules`, {
      name: `Webhook ${host}`,
      match: buildQuickMatch(),
      enabled: true,
    });
    if (!created.ok) {
      setSaving(false);
      toast.error(created.error);
      return;
    }
    const ruleId = created.data?.rule.id;
    if (!ruleId) {
      setSaving(false);
      toast.error("Falha ao salvar os filtros do webhook.");
      return;
    }

    const result = await bridgePost<{ webhook: Schema["WebhookView"]; secret: string }>(
      `/tenants/${tenantId}/webhooks`,
      {
        url: url.trim(),
        rule_id: ruleId,
        enabled: true,
        ...(Object.keys(customHeaders).length > 0 ? { headers: customHeaders } : {}),
      },
    );
    setSaving(false);

    if (!result.ok) {
      toast.error(result.error);
      return;
    }

    setUrl("");
    setHeaderText("");
    setOpen(false);
    setSecret(result.data?.secret ?? null);
    setSecretOpen(true);
    toast.success("Webhook criado.");
    router.refresh();
  }

  return (
    <>
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogTrigger render={<Button size="sm" />}>
          <Plus aria-hidden />
          Novo webhook
        </DialogTrigger>
        <DialogContent className="max-h-[90vh] overflow-y-auto sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>Novo webhook</DialogTitle>
            <DialogDescription>
              Os filtros viram uma regra reutilizável e o bridge entrega os eventos via POST assinado com HMAC.
              O secret aparece apenas uma vez.
            </DialogDescription>
          </DialogHeader>
          <form onSubmit={submit} className="space-y-3">
            <div className="space-y-1.5">
              <Label htmlFor="webhook-url">URL</Label>
              <Input
                id="webhook-url"
                type="url"
                value={url}
                onChange={(event) => setUrl(event.target.value)}
                placeholder="https://example.com/hooks/whatsapp"
                required
              />
            </div>

            <div className="space-y-1.5">
              <Label htmlFor="webhook-headers">Header de autenticação (opcional)</Label>
              <Textarea
                id="webhook-headers"
                value={headerText}
                onChange={(event) => setHeaderText(event.target.value)}
                placeholder={"Authorization: Bearer ...\nx-api-key: ..."}
                rows={2}
                className="min-h-14 font-mono text-xs"
              />
              <p className="text-xs text-muted-foreground">
                Um por linha, formato Nome: valor. Use para rotinas externas que exigem chave no header.
              </p>
            </div>

            <div className="space-y-2 rounded-md border p-3">
              <Label className="text-xs">Tipos de conversa</Label>
              <div className="flex flex-wrap gap-3">
                {CHAT_TYPES.map((option) => (
                  <label key={option.value} className="flex items-center gap-2 text-xs">
                    <input
                      type="checkbox"
                      checked={chatTypes.includes(option.value)}
                      onChange={() => setChatTypes((current) => toggleValue(current, option.value))}
                      className="size-3.5 accent-primary"
                    />
                    {option.label}
                  </label>
                ))}
              </div>
              <p className="text-xs text-muted-foreground">
                Privados por padrão. Libere grupos, status ou canais apenas quando necessário.
              </p>
            </div>

            <div className="space-y-2 rounded-md border p-3">
              <Label className="text-xs">Tipos de mensagem</Label>
              <div className="flex flex-wrap gap-2.5">
                {MESSAGE_TYPES.map((type) => (
                  <label key={type} className="flex items-center gap-1.5 text-xs">
                    <input
                      type="checkbox"
                      checked={messageTypes.includes(type)}
                      onChange={() => setMessageTypes((current) => toggleValue(current, type))}
                      className="size-3.5 accent-primary"
                    />
                    {type}
                  </label>
                ))}
              </div>
            </div>

            <div className="flex flex-wrap gap-3 rounded-md border p-3">
              <label className="flex items-center gap-2 text-xs">
                <input
                  type="checkbox"
                  checked={inbound}
                  onChange={(event) => setInbound(event.target.checked)}
                  className="size-3.5 accent-primary"
                />
                só recebidas
              </label>
              <label className="flex items-center gap-2 text-xs">
                <input
                  type="checkbox"
                  checked={mention}
                  onChange={(event) => setMention(event.target.checked)}
                  className="size-3.5 accent-primary"
                />
                só quando me marcam
              </label>
              <label className="flex items-center gap-2 text-xs">
                <input
                  type="checkbox"
                  checked={reply}
                  onChange={(event) => setReply(event.target.checked)}
                  className="size-3.5 accent-primary"
                />
                só respostas à minha mensagem
              </label>
            </div>

                <div className="grid gap-3 sm:grid-cols-2">
                  <div className="space-y-1.5">
                    <Label htmlFor="webhook-chats" className="text-xs">
                      chats específicos
                    </Label>
                    <Textarea
                      id="webhook-chats"
                      value={chatJids}
                      onChange={(event) => setChatJids(event.target.value)}
                      placeholder={"5511999999999@s.whatsapp.net\n1203630@g.us"}
                      rows={2}
                      className="min-h-14 font-mono text-xs"
                    />
                  </div>
                  <div className="space-y-1.5">
                    <Label htmlFor="webhook-senders" className="text-xs">
                      números específicos
                    </Label>
                    <Textarea
                      id="webhook-senders"
                      value={senderJids}
                      onChange={(event) => setSenderJids(event.target.value)}
                      placeholder={"5511888888888@s.whatsapp.net"}
                      rows={2}
                      className="min-h-14 font-mono text-xs"
                    />
                  </div>
                </div>

                <div className="space-y-1.5">
                  <Label htmlFor="webhook-contains" className="text-xs">
                    texto contém
                  </Label>
                  <Textarea
                    id="webhook-contains"
                    value={textContains}
                    onChange={(event) => setTextContains(event.target.value)}
                    placeholder={"urgente\npedido"}
                    rows={2}
                    className="min-h-14 text-xs"
                  />
                </div>
                <p className="text-xs text-muted-foreground">Sem nenhum filtro, o webhook recebe todos os eventos.</p>

            <DialogFooter showCloseButton>
              <Button type="submit" size="sm" disabled={saving}>
                {saving ? "Criando…" : "Criar webhook"}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      <SecretDialog
        open={secretOpen}
        onOpenChange={setSecretOpen}
        title="Secret do webhook"
        description="Use este secret para validar a assinatura HMAC das entregas."
        secret={secret}
      />
    </>
  );
}
