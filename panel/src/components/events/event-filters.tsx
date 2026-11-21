"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

export function EventFilters({ initialAfter }: { initialAfter: string }) {
  const router = useRouter();
  const [after, setAfter] = useState(initialAfter);

  function apply() {
    const params = new URLSearchParams();
    if (after.trim()) {
      params.set("after", after.trim());
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
            router.push("/events");
          }}
        >
          Limpar
        </Button>
      </div>
    </form>
  );
}
