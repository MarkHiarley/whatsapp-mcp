"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";

export type RuleOption = { id: string; name: string };

export function EventFilters({
  rules,
  initialAfter,
  initialRule,
}: {
  rules: RuleOption[];
  initialAfter: string;
  initialRule: string;
}) {
  const router = useRouter();
  const [after, setAfter] = useState(initialAfter);
  const [rule, setRule] = useState(initialRule || "all");

  const ruleItems: Record<string, string> = { all: "Todas as regras" };
  for (const option of rules) {
    ruleItems[option.id] = option.name;
  }

  function apply() {
    const params = new URLSearchParams();
    if (after.trim()) {
      params.set("after", after.trim());
    }
    if (rule !== "all") {
      params.set("rule", rule);
    }
    const query = params.toString();
    router.push(query ? `/events?${query}` : "/events");
  }

  return (
    <form
      className="flex flex-col gap-3 sm:flex-row sm:items-end"
      onSubmit={(event) => {
        event.preventDefault();
        apply();
      }}
    >
      <div className="space-y-1.5">
        <Label htmlFor="events-after">after (seq)</Label>
        <Input
          id="events-after"
          type="number"
          min={0}
          inputMode="numeric"
          value={after}
          onChange={(event) => setAfter(event.target.value)}
          placeholder="0"
          className="w-full sm:w-32"
        />
      </div>
      <div className="space-y-1.5">
        <Label>Regra</Label>
        <Select items={ruleItems} value={rule} onValueChange={(value) => setRule(value ?? "all")}>
          <SelectTrigger className="w-full sm:w-56">
            <SelectValue placeholder="Todas as regras" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">Todas as regras</SelectItem>
            {rules.map((option) => (
              <SelectItem key={option.id} value={option.id}>
                {option.name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
      <div className="flex gap-2">
        <Button type="submit" size="sm">
          Aplicar
        </Button>
        <Button
          type="button"
          variant="ghost"
          size="sm"
          onClick={() => {
            setAfter("");
            setRule("all");
            router.push("/events");
          }}
        >
          Limpar
        </Button>
      </div>
    </form>
  );
}
