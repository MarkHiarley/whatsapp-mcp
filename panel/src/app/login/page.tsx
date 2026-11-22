import { redirect } from "next/navigation";
import { Check } from "lucide-react";

import { BrandLogo } from "@/components/brand-logo";

import { GoogleSignInButton } from "@/components/auth/google-sign-in";
import { SignOutButton } from "@/components/auth/sign-out-button";
import { isEmailAllowed } from "@/lib/access";
import { getSession } from "@/lib/session";

export const instant = false;

const NOTES = ["Acesso restrito à allowlist do painel", "Keys ficam cifradas no servidor"];

export default async function LoginPage() {
  const session = await getSession();
  const allowed = session ? isEmailAllowed(session.user.email) : true;
  if (session && allowed) {
    redirect("/");
  }

  return (
    <main className="grid min-h-svh place-items-center bg-background px-4 py-10">
      <div className="w-full max-w-sm">
        <div className="flex items-center gap-2.5">
          <span className="flex size-9 items-center justify-center rounded-[0.7rem] bg-white p-1 ring-1 ring-black/5 dark:bg-[#f6f6f3]">
            <BrandLogo />
          </span>
          <div className="leading-tight">
            <p className="font-[family-name:var(--font-sora)] text-[13px] font-semibold tracking-tight">
              WhatsApp Bridge
            </p>
            <p className="text-[10px] font-medium tracking-[0.14em] text-muted-foreground uppercase">
              painel operacional
            </p>
          </div>
        </div>

        <div className="mt-6 rounded-lg border bg-card p-6 shadow-[0_1px_2px_rgb(23_23_20_/_0.04),0_1px_1px_rgb(23_23_20_/_0.03)]">
          {session && !allowed ? (
            <>
              <h1 className="font-[family-name:var(--font-sora)] text-2xl font-semibold tracking-tight">
                Acesso não autorizado
              </h1>
              <p className="mt-2 text-sm text-muted-foreground">
                A conta <span className="font-medium text-foreground">{session.user.email}</span> não está na allowlist
                do painel. Peça acesso ao administrador.
              </p>
              <div className="mt-6">
                <SignOutButton className="w-full" />
              </div>
            </>
          ) : (
            <>
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
            </>
          )}
        </div>

        <p className="mt-4 text-center text-[11px] text-muted-foreground">
          Nenhuma key do bridge chega ao navegador.
        </p>
      </div>
    </main>
  );
}
