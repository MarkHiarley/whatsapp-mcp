import { ShieldCheck, TriangleAlert } from "lucide-react";

import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";

export function NotConfigured() {
  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <TriangleAlert className="size-4 text-amber-500" aria-hidden />
          Painel sem configuração do bridge
        </CardTitle>
        <CardDescription>
          As credenciais ficam apenas no servidor. Nenhum token é enviado ao navegador.
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4 text-sm">
        <p className="text-muted-foreground">Defina as variáveis de ambiente e reinicie o painel:</p>
        <pre className="overflow-x-auto rounded-lg bg-muted p-3 font-mono text-xs leading-relaxed">
{`BRIDGE_URL=http://127.0.0.1:8080/api/v1
BRIDGE_TENANT_ID=default
BRIDGE_API_TOKEN=<token do tenant>
BRIDGE_ADMIN_TOKEN=<token admin, opcional>`}
        </pre>
        <div className="flex items-start gap-2 text-muted-foreground">
          <ShieldCheck className="mt-0.5 size-4 shrink-0" aria-hidden />
          <p>
            <code className="font-mono text-xs">BRIDGE_API_TOKEN</code> habilita o tenant configurado;{" "}
            <code className="font-mono text-xs">BRIDGE_ADMIN_TOKEN</code> habilita a tela de tenants.
          </p>
        </div>
      </CardContent>
    </Card>
  );
}

export function AdminNotConfigured({ message }: { message: string }) {
  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <TriangleAlert className="size-4 text-amber-500" aria-hidden />
          Administração indisponível
        </CardTitle>
        <CardDescription>{message}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-2 text-sm text-muted-foreground">
        <p>
          Configure <code className="font-mono text-xs">BRIDGE_ADMIN_TOKEN</code> no painel e{" "}
          <code className="font-mono text-xs">WHATSAPP_ADMIN_TOKEN</code> no bridge. Sem o token admin o bridge
          responde <code className="font-mono text-xs">503 ADMIN_DISABLED</code>.
        </p>
      </CardContent>
    </Card>
  );
}
