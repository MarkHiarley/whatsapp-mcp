"use client";

import { Copy } from "lucide-react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";

export function CopyButton({ value, label = "Copiar", className }: { value: string; label?: string; className?: string }) {
  async function copy() {
    try {
      await navigator.clipboard.writeText(value);
      toast.success("Copiado para a área de transferência.");
    } catch {
      toast.error("Não foi possível copiar. Selecione o texto manualmente.");
    }
  }

  return (
    <Button type="button" variant="outline" size="sm" className={className} onClick={copy}>
      <Copy aria-hidden />
      {label}
    </Button>
  );
}
