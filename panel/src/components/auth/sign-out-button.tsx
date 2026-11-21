"use client";

import { useRouter } from "next/navigation";
import { LogOut } from "lucide-react";

import { Button } from "@/components/ui/button";
import { authClient } from "@/lib/auth-client";

export function SignOutButton({ className, compact = false }: { className?: string; compact?: boolean }) {
  const router = useRouter();

  async function signOut() {
    await authClient.signOut();
    router.push("/login");
    router.refresh();
  }

  if (compact) {
    return (
      <Button variant="ghost" size="icon" className={className} onClick={signOut} type="button" aria-label="Sair">
        <LogOut className="size-3.5" aria-hidden />
      </Button>
    );
  }

  return (
    <Button variant="ghost" size="sm" className={className} onClick={signOut} type="button">
      <LogOut className="size-3.5" aria-hidden />
      Sair
    </Button>
  );
}
