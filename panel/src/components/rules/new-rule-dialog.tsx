"use client";

import { useRouter } from "next/navigation";
import { useState, type FormEvent } from "react";
import { Plus } from "lucide-react";
import { toast } from "sonner";

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

export function NewRuleDialog({ tenantId }: { tenantId: string }) {
  const router = useRouter();
  const [open, setOpen] = useState(false);
  const [saving, setSaving] = useState(false);
  const [name, setName] = useState("");
  const [chatJids, setChatJids] = useState("");
  const [senderJids, setSenderJids] = useState("");
  const [textContains, setTextContains] = useState("");
  const [textRegex, setTextRegex] = useState("");
  const [isGroup, setIsGroup] = useState(false);
  const [isFromMe, setIsFromMe] = useState(false);
  const [mentionedMe, setMentionedMe] = useState(false);

  function reset() {
    setName("");
    setChatJids("");
    setSenderJids("");
    setTextContains("");
    setTextRegex("");
    setIsGroup(false);
    setIsFromMe(false);
    setMentionedMe(false);
  }

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!name.trim()) {
      toast.error("Informe um nome para a regra.");
      return;
    }

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
    if (textRegex.trim()) {
      match.text_regex = textRegex.trim();
    }
    if (isGroup) {
      match.is_group = true;
    }
    if (isFromMe) {
      match.is_from_me = true;
    }
    if (mentionedMe) {
      match.mentioned_me = true;
    }

    setSaving(true);
    const result = await bridgePost<{ rule: Schema["EventRule"] }>(`/tenants/${tenantId}/rules`, {
      name: name.trim(),
      match,
      enabled: true,
    });
    setSaving(false);

    if (!result.ok) {
      toast.error(result.error);
      return;
    }

    toast.success(`Regra “${result.data?.rule.name ?? name.trim()}” criada.`);
    reset();
    setOpen(false);
    router.refresh();
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger render={<Button size="sm" />}>
        <Plus aria-hidden />
        Nova regra
      </DialogTrigger>
      <DialogContent className="max-h-[90vh] overflow-y-auto sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>Nova regra</DialogTitle>
          <DialogDescription>
            Campos vazios não entram no match. Listas aceitam um valor por linha (JID completo ou telefone).
          </DialogDescription>
        </DialogHeader>
        <form onSubmit={submit} className="space-y-3">
          <div className="space-y-1.5">
            <Label htmlFor="rule-name">Nome</Label>
            <Input id="rule-name" value={name} onChange={(event) => setName(event.target.value)} placeholder="Alertas do suporte" required />
          </div>
          <div className="grid gap-3 sm:grid-cols-2">
            <div className="space-y-1.5">
              <Label htmlFor="rule-chats">chat_jids</Label>
              <Textarea
                id="rule-chats"
                value={chatJids}
                onChange={(event) => setChatJids(event.target.value)}
                placeholder={"5511999999999@s.whatsapp.net\n1203630@g.us"}
                rows={3}
                className="min-h-20 font-mono text-xs"
              />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="rule-senders">sender_jids</Label>
              <Textarea
                id="rule-senders"
                value={senderJids}
                onChange={(event) => setSenderJids(event.target.value)}
                placeholder={"5511888888888@s.whatsapp.net"}
                rows={3}
                className="min-h-20 font-mono text-xs"
              />
            </div>
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="rule-contains">text_contains</Label>
            <Textarea
              id="rule-contains"
              value={textContains}
              onChange={(event) => setTextContains(event.target.value)}
              placeholder={"urgente\npedido"}
              rows={2}
              className="min-h-14"
            />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="rule-regex">text_regex</Label>
            <Input
              id="rule-regex"
              value={textRegex}
              onChange={(event) => setTextRegex(event.target.value)}
              placeholder="(?i)pedido\\s+#?\\d+"
              className="font-mono text-xs"
            />
          </div>
          <div className="flex flex-wrap gap-4 pt-1">
            <label className="flex items-center gap-2 text-sm">
              <input
                type="checkbox"
                checked={isGroup}
                onChange={(event) => setIsGroup(event.target.checked)}
                className="size-4 accent-primary"
              />
              is_group
            </label>
            <label className="flex items-center gap-2 text-sm">
              <input
                type="checkbox"
                checked={isFromMe}
                onChange={(event) => setIsFromMe(event.target.checked)}
                className="size-4 accent-primary"
              />
              is_from_me
            </label>
            <label className="flex items-center gap-2 text-sm">
              <input
                type="checkbox"
                checked={mentionedMe}
                onChange={(event) => setMentionedMe(event.target.checked)}
                className="size-4 accent-primary"
              />
              mentioned_me
            </label>
          </div>
          <DialogFooter showCloseButton>
            <Button type="submit" size="sm" disabled={saving}>
              {saving ? "Criando…" : "Criar regra"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
