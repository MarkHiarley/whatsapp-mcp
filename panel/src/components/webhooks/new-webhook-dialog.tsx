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
import type { Schema } from "@/lib/api";
import { bridgePost } from "@/lib/client-api";

export function NewWebhookDialog({ tenantId, rules }: { tenantId: string; rules: RuleOption[] }) {
  const router = useRouter();
  const [open, setOpen] = useState(false);
  const [saving, setSaving] = useState(false);
  const [url, setUrl] = useState("");
  const [rule, setRule] = useState(rules[0]?.id ?? "");
  const [secret, setSecret] = useState<string | null>(null);
  const [secretOpen, setSecretOpen] = useState(false);

  const ruleItems: Record<string, string> = {};
  for (const option of rules) {
    ruleItems[option.id] = option.name;
  }

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!url.trim() || !rule) {
      toast.error("Informe a URL e selecione uma regra.");
      return;
    }

    setSaving(true);
    const result = await bridgePost<{ webhook: Schema["WebhookView"]; secret: string }>(
      `/tenants/${tenantId}/webhooks`,
      { url: url.trim(), rule_id: rule, enabled: true },
    );
    setSaving(false);

    if (!result.ok) {
      toast.error(result.error);
      return;
    }

    setUrl("");
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
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Novo webhook</DialogTitle>
            <DialogDescription>
              O bridge entrega os eventos da regra via POST assinado com HMAC. O secret aparece apenas uma vez.
            </DialogDescription>
          </DialogHeader>
          {rules.length === 0 ? (
            <p className="text-sm text-muted-foreground">
              Crie uma regra antes de configurar um webhook.
            </p>
          ) : (
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
              <DialogFooter showCloseButton>
                <Button type="submit" size="sm" disabled={saving}>
                  {saving ? "Criando…" : "Criar webhook"}
                </Button>
              </DialogFooter>
            </form>
          )}
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
