"use client";

import { useRouter } from "next/navigation";
import { useState, type FormEvent } from "react";
import { Plus } from "lucide-react";
import { toast } from "sonner";

import type { RuleOption } from "@/components/events/event-filters";
import { SecretDialog } from "@/components/secret-dialog";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
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

const MEDIA_TYPES = ["image", "video", "audio", "document"] as const;
type MediaType = (typeof MEDIA_TYPES)[number];

export function NewWebhookDialog({ tenantId, rules }: { tenantId: string; rules: RuleOption[] }) {
  const router = useRouter();
  const [open, setOpen] = useState(false);
  const [saving, setSaving] = useState(false);
  const [url, setUrl] = useState("");
  const [headerText, setHeaderText] = useState("");
  const [mode, setMode] = useState<"quick" | "existing">("quick");
  const [rule, setRule] = useState(rules[0]?.id ?? "");
  const [chatJids, setChatJids] = useState("");
  const [senderJids, setSenderJids] = useState("");
  const [textContains, setTextContains] = useState("");
  const [conversation, setConversation] = useState<"any" | "group" | "direct">("any");
  const [inbound, setInbound] = useState(true);
  const [mention, setMention] = useState(false);
  const [reply, setReply] = useState(false);
  const [media, setMedia] = useState<"any" | "with" | "without">("any");
  const [mediaTypes, setMediaTypes] = useState<MediaType[]>([]);
  const [secret, setSecret] = useState<string | null>(null);
  const [secretOpen, setSecretOpen] = useState(false);

  const ruleItems: Record<string, string> = {};
  for (const option of rules) {
    ruleItems[option.id] = option.name;
  }

  function toggleMediaType(type: MediaType) {
    setMediaTypes((current) =>
      current.includes(type) ? current.filter((value) => value !== type) : [...current, type],
    );
  }

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
    if (conversation === "group") {
      match.is_group = true;
    } else if (conversation === "direct") {
      match.is_group = false;
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
    if (media !== "any") {
      match.has_media = media === "with";
    }
    if (media !== "without" && mediaTypes.length > 0) {
      match.media_types = mediaTypes;
    }
    return match;
  }

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!url.trim()) {
      toast.error("Informe a URL do webhook.");
      return;
    }
    if (mode === "existing" && !rule) {
      toast.error("Selecione uma regra.");
      return;
    }
    const parsedHeaders = parseHeaderLines(headerText);
    if (parsedHeaders.error) {
      toast.error(parsedHeaders.error);
      return;
    }
    const customHeaders = parsedHeaders.headers;

    setSaving(true);
    let ruleId = rule;
    if (mode === "quick") {
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
      const createdRuleID = created.data?.rule.id;
      if (!createdRuleID) {
        setSaving(false);
        toast.error("Falha ao criar a regra do webhook.");
        return;
      }
      ruleId = createdRuleID;
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

            {rules.length > 0 && (
              <div className="space-y-1.5">
                <Label htmlFor="webhook-mode">Filtro</Label>
                <select
                  id="webhook-mode"
                  value={mode}
                  onChange={(event) => setMode(event.target.value as "quick" | "existing")}
                  className="h-9 w-full rounded-md border bg-background px-2 text-sm"
                >
                  <option value="quick">Criar filtro aqui</option>
                  <option value="existing">Usar regra existente</option>
                </select>
              </div>
            )}

            {mode === "existing" && rules.length > 0 ? (
              <div className="space-y-1.5">
                <Label>Regra</Label>
                <Select items={ruleItems} value={rule} onValueChange={(value) => setRule(value ?? "")}>
                  <SelectTrigger className="w-full">
                    <SelectValue placeholder="Selecione uma regra" />
                  </SelectTrigger>
                  <SelectContent>
                    {rules.map((option) => (
                      <SelectItem key={option.id} value={option.id}>
                        {option.name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            ) : (
              <>
                <div className="space-y-2 rounded-md border p-3">
                  <div className="flex flex-wrap items-center justify-between gap-2">
                    <Label htmlFor="webhook-conversation" className="text-xs">
                      Conversa
                    </Label>
                    <select
                      id="webhook-conversation"
                      value={conversation}
                      onChange={(event) => setConversation(event.target.value as "any" | "group" | "direct")}
                      className="h-8 rounded-md border bg-background px-2 text-xs"
                    >
                      <option value="any">qualquer</option>
                      <option value="group">só grupos</option>
                      <option value="direct">só diretas</option>
                    </select>
                  </div>
                  <div className="flex flex-wrap gap-3">
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
                </div>

                <div className="space-y-2 rounded-md border p-3">
                  <div className="flex flex-wrap items-center justify-between gap-2">
                    <Label htmlFor="webhook-media" className="text-xs">
                      Mídia
                    </Label>
                    <select
                      id="webhook-media"
                      value={media}
                      onChange={(event) => setMedia(event.target.value as "any" | "with" | "without")}
                      className="h-8 rounded-md border bg-background px-2 text-xs"
                    >
                      <option value="any">qualquer</option>
                      <option value="with">com mídia</option>
                      <option value="without">sem mídia</option>
                    </select>
                  </div>
                  {media !== "without" && (
                    <div className="flex flex-wrap gap-3">
                      {MEDIA_TYPES.map((type) => (
                        <label key={type} className="flex items-center gap-2 text-xs">
                          <input
                            type="checkbox"
                            checked={mediaTypes.includes(type)}
                            onChange={() => toggleMediaType(type)}
                            className="size-3.5 accent-primary"
                          />
                          {type}
                        </label>
                      ))}
                    </div>
                  )}
                  {media === "without" && (
                    <p className="text-xs text-muted-foreground">Sem mídia ignora os tipos selecionados.</p>
                  )}
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
              </>
            )}

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
