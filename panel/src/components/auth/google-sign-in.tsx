"use client";

import { useState } from "react";

import { authClient } from "@/lib/auth-client";
import { Button } from "@/components/ui/button";

export function GoogleSignInButton() {
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function signIn() {
    setPending(true);
    setError(null);
    const { error: signInError } = await authClient.signIn.social({
      provider: "google",
      callbackURL: "/onboarding",
    });
    if (signInError) {
      setError(signInError.message ?? "Não foi possível iniciar o login com Google.");
      setPending(false);
    }
  }

  return (
    <div className="space-y-2">
      <Button className="w-full" size="lg" type="button" onClick={signIn} disabled={pending}>
        {pending ? "Redirecionando…" : "Continuar com Google"}
      </Button>
      {error ? <p className="text-xs text-destructive">{error}</p> : null}
    </div>
  );
}
