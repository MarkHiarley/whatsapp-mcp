import { TriangleAlert } from "lucide-react";

import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";

export function ErrorState({ message, code }: { message: string; code?: string }) {
  return (
    <Card className="border-destructive/30">
      <CardHeader>
        <CardTitle className="flex items-center gap-2 text-destructive">
          <TriangleAlert className="size-4" aria-hidden />
          Não foi possível carregar os dados
        </CardTitle>
        <CardDescription>{message}</CardDescription>
      </CardHeader>
      {code ? (
        <CardContent>
          <code className="rounded bg-muted px-1.5 py-0.5 font-mono text-xs text-muted-foreground">{code}</code>
        </CardContent>
      ) : null}
    </Card>
  );
}
