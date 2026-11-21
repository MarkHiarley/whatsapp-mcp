"use client";

import { useRouter } from "next/navigation";
import { useState, type FormEvent } from "react";
import { toast } from "sonner";

import type { RuleOption } from "@/components/events/event-filters";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { bridgePost } from "@/lib/client-api";

export function AckForm({
  tenantId,
  rules,
  defaultRule,
  defaultSeq,
}: {
  tenantId: string;
  rules: RuleOption[];
  defaultRule: string;
  defaultSeq: number;
}) {
  const router = useRouter();
  const [consumer, setConsumer] = useState("");
  const [rule, setRule] = useState(defaultRule || rules[0]?.id || "");
  const [seq, setSeq] = useState(String(defaultSeq));
  const [sending, setSending] = useState(false);

  const ruleItems: Record<string, string> = {};
  for (const option of rules) {
    ruleItems[option.id] = option.name;
  }

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const consumerId = consumer.trim();
    const parsedSeq = Number.parseInt(seq, 10);
    if (!consumerId) {
      toast.error("Informe o id do consumer.");
      return;
    }
    if (!rule) {
      toast.error("Selecione uma regra para o ack.");
      return;
    }
    if (!Number.isFinite(parsedSeq) || parsedSeq < 0) {
      toast.error("Informe um seq válido.");
      return;
    }

    setSending(true);
    const result = await bridgePost<{ consumer_id: string; last_seq: number }>(
      `/tenants/${tenantId}/consumers/${encodeURIComponent(consumerId)}/ack`,
      { rule_id: rule, seq: parsedSeq },
    );
    setSending(false);

    if (!result.ok) {
      toast.error(result.error);
      return;
    }
    toast.success(`Cursor de ${result.data?.consumer_id ?? consumerId} salvo em ${result.data?.last_seq ?? parsedSeq}.`);
    router.refresh();
  }

  return (
    <form onSubmit={submit} className="flex flex-col gap-3 sm:flex-row sm:items-end">
      <div className="space-y-1.5">
        <Label htmlFor="ack-consumer">Consumer</Label>
        <Input
          id="ack-consumer"
          value={consumer}
          onChange={(event) => setConsumer(event.target.value)}
          placeholder="meu-worker"
          className="w-full sm:w-44"
          required
        />
      </div>
      <div className="space-y-1.5">
        <Label>Regra</Label>
        <Select items={ruleItems} value={rule} onValueChange={(value) => setRule(value ?? "")}>
          <SelectTrigger className="w-full sm:w-52" disabled={rules.length === 0}>
            <SelectValue placeholder="Selecione" />
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
      <div className="space-y-1.5">
        <Label htmlFor="ack-seq">seq</Label>
        <Input
          id="ack-seq"
          type="number"
          min={0}
          value={seq}
          onChange={(event) => setSeq(event.target.value)}
          className="w-full sm:w-28"
        />
      </div>
      <Button type="submit" size="sm" disabled={sending || rules.length === 0}>
        {sending ? "Salvando…" : "Enviar ack"}
      </Button>
    </form>
  );
}
