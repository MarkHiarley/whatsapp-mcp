"use client";

import type { ReactNode } from "react";

import { CopyButton } from "@/components/copy-button";
import { Button } from "@/components/ui/button";
import { Dialog, DialogClose, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";

export function SecretDialog({
  open,
  onOpenChange,
  title,
  description,
  secret,
  children,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  description: string;
  secret: string | null;
  children?: ReactNode;
}) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription>{description}</DialogDescription>
        </DialogHeader>
        {secret ? (
          <div className="space-y-3">
            <pre className="max-h-40 overflow-auto rounded-lg bg-muted p-3 font-mono text-xs break-all whitespace-pre-wrap">
              {secret}
            </pre>
            <p className="text-xs text-muted-foreground">
              Esse valor não pode ser recuperado depois. Guarde-o em um gerenciador de segredos.
            </p>
          </div>
        ) : (
          <p className="text-sm text-muted-foreground">Nenhum valor disponível.</p>
        )}
        <DialogFooter>
          <DialogClose render={<Button variant="outline" size="sm" />}>Fechar</DialogClose>
          {secret ? <CopyButton value={secret} /> : null}
          {children}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
