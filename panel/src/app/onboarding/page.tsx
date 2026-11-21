import { redirect } from "next/navigation";
import { MessagesSquare } from "lucide-react";

import { OnboardingForm } from "@/components/auth/onboarding-form";
import { SignOutButton } from "@/components/auth/sign-out-button";
import { getSession, getTenantContext, isPanelAdmin } from "@/lib/session";

export const instant = false;

export default async function OnboardingPage() {
  const session = await getSession();
  if (!session) {
    redirect("/login");
  }
  const context = await getTenantContext();
  if (context) {
    redirect("/");
  }
  const isAdmin = await isPanelAdmin(session.user.email, session.user.id);

  return (
    <main className="grid min-h-svh place-items-center bg-background px-4 py-10">
      <div className="w-full max-w-lg">
        <div className="flex items-center justify-between gap-3">
          <div className="flex items-center gap-2.5">
            <span className="flex size-9 items-center justify-center rounded-xl bg-primary text-primary-foreground">
              <MessagesSquare className="size-4" aria-hidden />
            </span>
            <div className="leading-tight">
              <p className="text-sm font-semibold">WhatsApp Bridge</p>
              <p className="text-[11px] text-muted-foreground">painel operacional</p>
            </div>
          </div>
          <SignOutButton />
        </div>

        <div className="mt-6 rounded-2xl border bg-card p-6">
          <h1 className="font-heading text-2xl font-semibold tracking-tight">Conectar um número</h1>
          <p className="mt-2 text-sm text-muted-foreground">
            Sua conta <span className="font-medium text-foreground">{session.user.email}</span> ainda não tem um tenant
            vinculado. Conecte uma key existente{isAdmin ? " ou crie um tenant novo" : ""}.
          </p>
          <div className="mt-5">
            <OnboardingForm isAdmin={isAdmin} />
          </div>
        </div>

        <p className="mt-4 px-1 text-[11px] text-muted-foreground">
          A key fica cifrada no Postgres do painel e é usada só no servidor para falar com o bridge.
        </p>
      </div>
    </main>
  );
}
