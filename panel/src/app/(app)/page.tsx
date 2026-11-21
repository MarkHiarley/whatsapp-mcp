import Link from "next/link";
import { connection } from "next/server";
import { Activity, ArrowRight, CircleAlert, Clock, KeyRound, MessageCircle, MessagesSquare, Paperclip, QrCode } from "lucide-react";

import { EmptyState } from "@/components/empty-state";
import { ErrorState } from "@/components/error-state";
import { MessagesChart } from "@/components/overview/messages-chart";
import { PageHeader } from "@/components/page-header";
import { StatusBadge } from "@/components/status-badge";
import { StatusOrb } from "@/components/visual/status-orb";
import { Avatar, AvatarFallback } from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
import { buttonVariants } from "@/components/ui/button";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { loadChats, loadStats, loadStatus } from "@/lib/api";
import { formatNumber, formatRelative, initials, mediaLabel, phoneFromJid } from "@/lib/format";
import { STATUS_LABELS } from "@/lib/status";

export const instant = false;

export default async function OverviewPage() {
  await connection();

  const [statsResult, statusResult, chatsResult] = await Promise.all([
    loadStats(),
    loadStatus(),
    loadChats(undefined, 5),
  ]);

  if (!statsResult.ok) {
    return (
      <>
        <PageHeader title="Visão geral" description="Sessão, volume e atividade do tenant configurado." />
        <ErrorState message={statsResult.error} code={statsResult.code} />
      </>
    );
  }

  const stats = statsResult.data;
  const status = statusResult.ok ? statusResult.data.status : stats.status;
  const jid = statusResult.ok ? statusResult.data.jid : undefined;
  const needsPairing = status === "disconnected" || status === "logged_out";
  const recentChats = chatsResult.ok ? chatsResult.data : [];

  const highlights = [
    { label: "Mensagens", value: formatNumber(stats.messages), hint: "sincronizadas no bridge", icon: MessageCircle },
    { label: "Eventos", value: formatNumber(stats.events), hint: "publicados para agentes", icon: Activity },
  ];

  const metrics = [
    { label: "Chats", value: formatNumber(stats.chats), hint: "no histórico", icon: MessagesSquare },
    { label: "Chaves", value: formatNumber(stats.keys), hint: "credenciais ativas", icon: KeyRound },
    { label: "Última atividade", value: formatRelative(stats.last_message_time), hint: "no bridge", icon: Clock },
  ];

  return (
    <>
      <PageHeader
        title="Visão geral"
        description="Sessão, volume e atividade do tenant configurado."
        actions={
          <Link href="/chats" className={buttonVariants({ variant: "outline", size: "sm" })}>
            Abrir conversas
            <ArrowRight aria-hidden />
          </Link>
        }
      />

      <section aria-label="Status da sessão" className="rounded-lg border bg-card p-5 shadow-xs sm:p-6">
        <div className="flex flex-col items-center gap-5 text-center sm:flex-row sm:items-center sm:text-left">
          <StatusOrb status={status} size={96} className="shrink-0" />
          <div className="min-w-0 flex-1">
            <div className="flex flex-wrap items-center justify-center gap-2 sm:justify-start">
              <StatusBadge status={status} />
              <span className="font-mono text-[11px] text-muted-foreground">{stats.tenant_id}</span>
            </div>
            <p className="mt-3 truncate font-mono text-xs text-muted-foreground">
              {jid ?? "JID indisponível até o pareamento"}
            </p>
            <p className="mt-1.5 text-sm">
              {STATUS_LABELS[status]}
              <span className="text-muted-foreground"> · última atividade {formatRelative(stats.last_message_time)}</span>
            </p>
          </div>
          <div className="flex flex-wrap justify-center gap-2 sm:ml-auto sm:justify-end">
            <Link href="/chats" className={buttonVariants({ size: "sm" })}>
              <MessagesSquare aria-hidden />
              Ver conversas
            </Link>
            <Link href="/settings" className={buttonVariants({ variant: "outline", size: "sm" })}>
              <QrCode aria-hidden />
              {needsPairing ? "Parear agora" : "Pareamento"}
            </Link>
          </div>
        </div>
        {!statusResult.ok ? (
          <p className="mt-4 border-t pt-3 text-xs text-muted-foreground">Status indisponível: {statusResult.error}</p>
        ) : null}
      </section>

      {needsPairing ? (
        <div
          role="status"
          className="mt-3 flex flex-col gap-2 rounded-lg border border-amber-500/40 bg-amber-500/10 px-3 py-2 text-sm sm:flex-row sm:items-center sm:justify-between"
        >
          <span className="flex items-center gap-2">
            <CircleAlert className="size-4 text-amber-600" aria-hidden />
            O número está {STATUS_LABELS[status]}.
          </span>
          <Link href="/settings" className="text-xs font-medium underline underline-offset-4 hover:text-foreground">
            Abrir pareamento
          </Link>
        </div>
      ) : null}

      <div className="mt-3 grid gap-3 sm:grid-cols-2">
        {highlights.map((highlight) => (
          <section key={highlight.label} className="rounded-lg border bg-card p-5 shadow-xs">
            <p className="flex items-center gap-1.5 text-xs font-medium text-muted-foreground">
              <highlight.icon className="size-3.5" aria-hidden />
              {highlight.label}
            </p>
            <p className="mt-3 text-4xl font-semibold tabular-nums">{highlight.value}</p>
            <p className="mt-1 text-xs text-muted-foreground">{highlight.hint}</p>
          </section>
        ))}
      </div>

      <dl aria-label="Volume do tenant" className="mt-3 grid grid-cols-1 gap-px overflow-hidden rounded-lg border bg-border shadow-xs sm:grid-cols-3">
        {metrics.map((metric) => (
          <div key={metric.label} className="bg-card px-5 py-4">
            <dt className="flex items-center gap-1.5 text-[11px] font-medium text-muted-foreground">
              <metric.icon className="size-3.5" aria-hidden />
              {metric.label}
            </dt>
            <dd className="mt-2 text-xl font-semibold tabular-nums">{metric.value}</dd>
            <p className="mt-1 text-[11px] text-muted-foreground">{metric.hint}</p>
          </div>
        ))}
      </dl>

      <section aria-labelledby="daily-volume" className="mt-3 rounded-lg border bg-card p-5 shadow-xs">
        <div className="mb-4 flex items-baseline justify-between gap-3">
          <h2 id="daily-volume" className="font-heading text-base font-semibold">
            Mensagens por dia
          </h2>
          <span className="text-xs text-muted-foreground">últimos 14 dias</span>
        </div>
        <MessagesChart data={stats.messages_by_day} />
      </section>

      <div className="mt-3 grid gap-3 xl:grid-cols-[minmax(0,2fr)_minmax(0,1fr)]">
        <section aria-labelledby="recent-activity" className="rounded-lg border bg-card p-5 shadow-xs">
          <div className="mb-3 flex items-baseline justify-between gap-3">
            <h2 id="recent-activity" className="font-heading text-base font-semibold">
              Atividade recente
            </h2>
            <Link
              href="/chats"
              className="text-xs text-muted-foreground underline-offset-4 hover:text-foreground hover:underline"
            >
              Ver todos
            </Link>
          </div>

          {!chatsResult.ok ? (
            <p className="text-sm text-muted-foreground">Não foi possível carregar as conversas: {chatsResult.error}</p>
          ) : recentChats.length === 0 ? (
            <EmptyState
              title="Nenhuma conversa sincronizada"
              description={
                needsPairing
                  ? "Pareie o número para começar a receber mensagens."
                  : "As conversas aparecem aqui assim que o bridge sincronizar o histórico."
              }
              action={
                <Link
                  href={needsPairing ? "/settings" : "/chats"}
                  className={buttonVariants({ variant: "outline", size: "sm" })}
                >
                  {needsPairing ? "Parear / QR" : "Abrir conversas"}
                </Link>
              }
            />
          ) : (
            <Table className="w-full table-fixed">
              <TableHeader>
                <TableRow className="hover:bg-transparent">
                  <TableHead className="h-9 px-2">Conversa</TableHead>
                  <TableHead className="hidden h-9 w-[42%] px-2 lg:table-cell">Última mensagem</TableHead>
                  <TableHead className="hidden h-9 w-[92px] px-2 text-right sm:table-cell">Quando</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {recentChats.map((chat) => (
                  <TableRow key={chat.jid} className="group">
                    <TableCell className="px-2 py-3">
                      <div className="flex items-center gap-3">
                        <Avatar className="size-9">
                          <AvatarFallback className="bg-accent text-[11px] font-medium text-accent-foreground">
                            {initials(chat.name || phoneFromJid(chat.jid))}
                          </AvatarFallback>
                        </Avatar>
                        <div className="min-w-0">
                          <div className="flex items-center gap-1.5">
                            <Link
                              href={`/chats/${encodeURIComponent(chat.jid)}`}
                              className="truncate text-sm font-medium hover:underline"
                            >
                              {chat.name || phoneFromJid(chat.jid)}
                            </Link>
                            {chat.is_group ? (
                              <Badge variant="secondary" className="h-4 shrink-0 px-1 text-[9px] tracking-wide">
                                grupo
                              </Badge>
                            ) : null}
                          </div>
                          <span className="block truncate font-mono text-[11px] text-muted-foreground">{chat.jid}</span>
                          <span className="mt-0.5 block text-[11px] text-muted-foreground sm:hidden">
                            {formatRelative(chat.last_message_time)}
                          </span>
                        </div>
                      </div>
                    </TableCell>
                    <TableCell className="hidden max-w-md px-2 py-3 lg:table-cell">
                      <span className="flex min-w-0 items-center gap-1.5 text-sm text-muted-foreground">
                        {chat.last_media_type ? (
                          <>
                            <Paperclip className="size-3.5 shrink-0" aria-hidden />
                            <span className="truncate">{mediaLabel(chat.last_media_type, null)}</span>
                          </>
                        ) : (
                          <span className="truncate">{chat.last_message || "—"}</span>
                        )}
                      </span>
                    </TableCell>
                    <TableCell className="hidden px-2 py-3 text-right text-xs text-muted-foreground sm:table-cell">
                      {formatRelative(chat.last_message_time)}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </section>

        <section aria-labelledby="session-details" className="rounded-lg border bg-card p-5 shadow-xs">
          <h2 id="session-details" className="font-heading text-base font-semibold">
            Sessão
          </h2>
          <dl className="mt-4 divide-y">
            <div className="flex items-center justify-between gap-3 py-2.5">
              <dt className="text-sm text-muted-foreground">Status</dt>
              <dd>
                <StatusBadge status={status} />
              </dd>
            </div>
            <div className="flex items-center justify-between gap-3 py-2.5">
              <dt className="text-sm text-muted-foreground">JID</dt>
              <dd className="max-w-[60%] truncate font-mono text-xs">{jid ?? "—"}</dd>
            </div>
            <div className="flex items-center justify-between gap-3 py-2.5">
              <dt className="text-sm text-muted-foreground">Chaves ativas</dt>
              <dd className="text-sm tabular-nums">{stats.keys}</dd>
            </div>
            <div className="flex items-center justify-between gap-3 py-2.5">
              <dt className="text-sm text-muted-foreground">Última atividade</dt>
              <dd className="text-sm">{formatRelative(stats.last_message_time)}</dd>
            </div>
          </dl>
          <Link
            href="/settings"
            className="mt-4 inline-flex w-full items-center justify-center gap-1.5 rounded-lg border px-3 py-2 text-xs font-medium transition-colors hover:bg-muted"
          >
            Gerenciar sessão
          </Link>
        </section>
      </div>
    </>
  );
}
