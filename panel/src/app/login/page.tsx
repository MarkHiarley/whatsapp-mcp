import { redirect } from "next/navigation";
import { Check, MessagesSquare } from "lucide-react";

import { GoogleSignInButton } from "@/components/auth/google-sign-in";
import { getSession } from "@/lib/session";

export const instant = false;

const NOTES = ["Acesso restrito à organização vinculada", "Keys ficam cifradas no servidor"];

export default async function LoginPage() {
  const session = await getSession();
  if (session) {
    redirect("/");
  }

  return (
    <main className="grid min-h-svh place-items-center bg-background px-4 py-10">
      <div className="w-full max-w-sm">
        <div className="flex items-center gap-2.5">
          <span className="flex size-9 items-center justify-center rounded-md bg-primary text-primary-foreground">
            <MessagesSquare className="size-4" aria-hidden />
          </span>
          <div className="leading-tight">
            <p className="text-sm font-semibold">WhatsApp Bridge</p>
            <p className="text-[11px] text-muted-foreground">painel operacional</p>
          </div>
        </div>

        <div className="mt-6 rounded-lg border bg-card p-6 shadow-[0_1px_2px_rgb(23_23_20_/_0.04),0_1px_1px_rgb(23_23_20_/_0.03)]">
          <h1 className="font-[family-name:var(--font-sora)] text-2xl font-semibold tracking-tight">Entrar</h1>
          <p className="mt-2 text-sm text-muted-foreground">
            Use sua conta Google para acessar o tenant vinculado.
          </p>

          <div className="mt-6">
            <GoogleSignInButton />
          </div>

          <ul className="mt-5 space-y-2 border-t pt-4 text-[11px] text-muted-foreground">
            {NOTES.map((note) => (
              <li key={note} className="flex items-center gap-2">
                <Check className="size-3 shrink-0" aria-hidden />
                {note}
              </li>
            ))}
          </ul>
        </div>

        <p className="mt-4 text-center text-[11px] text-muted-foreground">
          Nenhuma key do bridge chega ao navegador.
        </p>
      </div>
    </main>
  );
}
