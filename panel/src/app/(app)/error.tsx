"use client";

import { RotateCw, TriangleAlert } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";

export default function AppError({ error, reset }: { error: Error & { digest?: string }; reset: () => void }) {
  return (
    <Card className="border-destructive/30">
      <CardHeader>
        <CardTitle className="flex items-center gap-2 text-destructive">
          <TriangleAlert className="size-4" aria-hidden />
          Erro inesperado no painel
        </CardTitle>
        <CardDescription>{error.message || "A página falhou ao renderizar."}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-3">
        {error.digest ? (
          <p className="font-mono text-xs text-muted-foreground">digest: {error.digest}</p>
        ) : null}
        <Button type="button" variant="outline" size="sm" onClick={reset}>
          <RotateCw aria-hidden />
          Tentar novamente
        </Button>
      </CardContent>
    </Card>
  );
}
