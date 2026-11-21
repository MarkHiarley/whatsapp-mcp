import type { Metadata } from "next";
import { Geist, Geist_Mono } from "next/font/google";
import { connection } from "next/server";

import { AppShell } from "@/components/app-shell";
import { RefreshButton } from "@/components/refresh-button";
import { StatusBadge } from "@/components/status-badge";
import { ThemeProvider } from "@/components/theme-provider";
import { ThemeToggle } from "@/components/theme-toggle";
import { Badge } from "@/components/ui/badge";
import { Toaster } from "@/components/ui/sonner";
import { loadStatus } from "@/lib/api";
import { BRIDGE_URL, TENANT_ID, bridgeConfigured } from "@/lib/bridge";
import "./globals.css";

const geistSans = Geist({
  variable: "--font-geist-sans",
  subsets: ["latin"],
});

const geistMono = Geist_Mono({
  variable: "--font-geist-mono",
  subsets: ["latin"],
});

export const metadata: Metadata = {
  title: "Painel · WhatsApp Bridge",
  description: "Painel operacional multi-tenant do WhatsApp Bridge",
};

export const instant = false;

export default async function RootLayout({ children }: LayoutProps<"/">) {
  await connection();

  const configured = bridgeConfigured();
  const statusResult = configured ? await loadStatus() : null;
  const status = statusResult?.ok ? statusResult.data.status : null;
  const statusError = statusResult && !statusResult.ok ? statusResult.error : undefined;

  const header = (
    <>
      {configured ? (
        status ? (
          <StatusBadge status={status} />
        ) : (
          <Badge variant="destructive" title={statusError}>
            status indisponível
          </Badge>
        )
      ) : (
        <Badge variant="outline">sem configuração</Badge>
      )}
      {configured ? <RefreshButton /> : null}
      <ThemeToggle />
    </>
  );

  return (
    <html lang="pt-BR" suppressHydrationWarning className={`${geistSans.variable} ${geistMono.variable} h-full`}>
      <body className="min-h-full antialiased">
        <ThemeProvider>
          <AppShell header={header} tenant={TENANT_ID} status={status} bridgeUrl={BRIDGE_URL}>
            {children}
          </AppShell>
          <Toaster position="top-right" richColors />
        </ThemeProvider>
      </body>
    </html>
  );
}
