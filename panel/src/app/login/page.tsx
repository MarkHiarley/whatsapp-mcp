import { redirect } from "next/navigation";
import { MessagesSquare } from "lucide-react";

import { GoogleSignInButton } from "@/components/auth/google-sign-in";
import { getSession } from "@/lib/session";

export const instant = false;

export default async function LoginPage() {
  const session = await getSession();
  if (session) {
    redirect("/");
  }

  return (
    <main className="grid min-h-svh place-items-center bg-background px-4 py-10">
      <div className="w-full max-w-sm">
        <div className="flex items-center gap-2.5">
          <span className="flex size-9 items-center justify-center rounded-xl bg-primary text-primary-foreground">
            <MessagesSquare className="size-4" aria-hidden />
          </span>
          <div className="leading-tight">
            <p className="text-sm font-semibold">WhatsApp Bridge</p>
            <p className="text-[11px] text-muted-foreground">painel operacional</p>
          </div>
        </div>

        <div className="mt-6 rounded-2xl border bg-card p-6">
          <h1 className="font-heading text-2xl font-semibold tracking-tight">Entrar no painel</h1>
          <p className="mt-2 text-sm text-muted-foreground">Use sua conta Google para acessar o tenant vinculado.</p>
          <div className="mt-6">
            <GoogleSignInButton />
          </div>
          <p className="mt-4 text-[11px] leading-relaxed text-muted-foreground">
            O acesso é limitado à organização vinculada à sua conta. Nenhuma key do bridge chega ao navegador.
          </p>
        </div>

        <p className="mt-4 px-1 text-[11px] text-muted-foreground">
          Ambiente local · {process.env.BETTER_AUTH_URL ?? "localhost:3000"}
        </p>
      </div>
    </main>
  );
}
