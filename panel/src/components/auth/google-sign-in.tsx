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
        {pending ? (
          "Redirecionando…"
        ) : (
          <>
            <GoogleMark />
            Continuar com Google
          </>
        )}
      </Button>
      {error ? <p className="text-xs text-destructive">{error}</p> : null}
    </div>
  );
}

function GoogleMark() {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" aria-hidden>
      <path
        fill="#4285F4"
        d="M23.49 12.27c0-.79-.07-1.54-.19-2.27H12v4.51h6.47a5.54 5.54 0 0 1-2.4 3.64v3.02h3.87c2.27-2.09 3.55-5.17 3.55-8.9Z"
      />
      <path
        fill="#34A853"
        d="M12 24c3.24 0 5.95-1.08 7.93-2.91l-3.87-3.02c-1.08.72-2.45 1.15-4.06 1.15-3.13 0-5.78-2.11-6.73-4.96H1.28v3.09A12 12 0 0 0 12 24Z"
      />
      <path
        fill="#FBBC05"
        d="M5.27 14.26a7.2 7.2 0 0 1 0-4.52V6.65H1.28a12 12 0 0 0 0 10.7l3.99-3.09Z"
      />
      <path
        fill="#EA4335"
        d="M12 4.78c1.77 0 3.35.61 4.6 1.8l3.44-3.44A12 12 0 0 0 1.28 6.65l3.99 3.09C6.22 6.89 8.87 4.78 12 4.78Z"
      />
    </svg>
  );
}
