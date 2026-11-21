import { redirect } from "next/navigation";
import { Check, MessagesSquare } from "lucide-react";

import { GoogleSignInButton } from "@/components/auth/google-sign-in";
import { StatusOrb } from "@/components/visual/status-orb";
import { getSession } from "@/lib/session";

export const instant = false;

const HIGHLIGHTS = [
  "Vários números, sessões isoladas",
  "Eventos duráveis com cursor e regras",
  "Webhooks assinados com retry e DLQ",
];

export default async function LoginPage() {
  const session = await getSession();
  if (session) {
    redirect("/");
  }

  return (
    <main className="grid min-h-svh bg-background lg:grid-cols-[1.1fr_1fr]">
      <section className="relative hidden flex-col justify-between overflow-hidden bg-[#141126] p-10 text-white lg:flex">
        <div className="flex items-center gap-2.5">
          <span className="flex size-9 items-center justify-center rounded-xl bg-[#463ebc] text-white">
            <MessagesSquare className="size-4" aria-hidden />
          </span>
          <div className="leading-tight">
            <p className="font-heading text-sm font-semibold">WhatsApp Bridge</p>
            <p className="text-[11px] text-white/50">painel operacional</p>
          </div>
        </div>

        <div className="max-w-lg">
          <StatusOrb status="connected" size={190} label="Sinal do bridge" className="mb-8" />
          <h1 className="font-heading text-4xl font-semibold tracking-tight text-balance">
            Sinais de vários números, num só lugar.
          </h1>
          <p className="mt-4 max-w-md text-sm leading-relaxed text-white/60">
            Cada conta com sua sessão, suas regras e seus webhooks. O agente consome o que importa — não o ruído.
          </p>
          <ul className="mt-8 space-y-3 text-sm text-white/70">
            {HIGHLIGHTS.map((item) => (
              <li key={item} className="flex items-center gap-2.5">
                <span className="flex size-5 items-center justify-center rounded-full bg-white/10">
                  <Check className="size-3" aria-hidden />
                </span>
                {item}
              </li>
            ))}
          </ul>
        </div>

        <p className="text-[11px] text-white/35">
          Ambiente local · {process.env.BETTER_AUTH_URL ?? "localhost:3000"}
        </p>
      </section>

      <section className="flex flex-col justify-center px-6 py-10 sm:px-10 lg:px-14">
        <div className="mx-auto w-full max-w-sm">
          <div className="flex items-center gap-2.5 lg:hidden">
            <span className="flex size-9 items-center justify-center rounded-xl bg-primary text-primary-foreground">
              <MessagesSquare className="size-4" aria-hidden />
            </span>
            <div className="leading-tight">
              <p className="font-heading text-sm font-semibold">WhatsApp Bridge</p>
              <p className="text-[11px] text-muted-foreground">painel operacional</p>
            </div>
          </div>

          <h2 className="mt-8 font-heading text-3xl font-semibold tracking-tight">Entrar</h2>
          <p className="mt-2 text-sm text-muted-foreground">Use sua conta Google para acessar o tenant vinculado.</p>

          <div className="mt-6">
            <GoogleSignInButton />
          </div>

          <p className="mt-4 text-[11px] leading-relaxed text-muted-foreground">
            O acesso é limitado à organização vinculada à sua conta. Nenhuma key do bridge chega ao navegador.
          </p>

          <div className="mt-10 flex items-center gap-2 border-t pt-4 text-[11px] text-muted-foreground">
            <span className="size-1.5 rounded-full bg-emerald-500" aria-hidden />
            Bridge em {process.env.BRIDGE_URL ?? "127.0.0.1:8080"}
          </div>
        </div>
      </section>
    </main>
  );
}
