import Link from "next/link";
import { connection } from "next/server";
import { ArrowRight } from "lucide-react";

import { AckForm } from "@/components/events/ack-form";
import { EventFilters } from "@/components/events/event-filters";
import { EmptyState } from "@/components/empty-state";
import { ErrorState } from "@/components/error-state";
import { NotConfigured } from "@/components/not-configured";
import { PageHeader } from "@/components/page-header";
import { Badge } from "@/components/ui/badge";
import { buttonVariants } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { loadEvents, loadRules } from "@/lib/api";
import { TENANT_ID, bridgeConfigured } from "@/lib/bridge";
import { formatDateTime, truncate } from "@/lib/format";

export const instant = false;

function parseAfter(value?: string): number | undefined {
  if (!value) {
    return undefined;
  }
  const parsed = Number.parseInt(value, 10);
  return Number.isFinite(parsed) && parsed >= 0 ? parsed : undefined;
}

export default async function EventsPage({
  searchParams,
}: {
  searchParams: Promise<{ after?: string; rule?: string }>;
}) {
  await connection();

  if (!bridgeConfigured()) {
    return (
      <>
        <PageHeader title="Eventos" description="Pull durável de eventos por cursor." />
        <NotConfigured />
      </>
    );
  }

  const { after: afterParam, rule: ruleParam } = await searchParams;
  const after = parseAfter(afterParam);
  const rule = ruleParam?.trim() || undefined;

  const [eventsResult, rulesResult] = await Promise.all([loadEvents({ after, rule }), loadRules()]);
  const rules = rulesResult.ok ? rulesResult.data.map((item) => ({ id: item.id, name: item.name })) : [];
  const events = eventsResult.ok ? eventsResult.data.events : [];
  const nextAfter = eventsResult.ok ? eventsResult.data.nextAfter : 0;

  const advanceParams = new URLSearchParams();
  advanceParams.set("after", String(nextAfter));
  if (rule) {
    advanceParams.set("rule", rule);
  }

  return (
    <>
      <PageHeader
        title="Eventos"
        description="Eventos em ordem de seq. Use o cursor para consumir sem perder nada (at-least-once)."
      />

      <Card className="mb-4">
        <CardHeader>
          <CardTitle className="text-sm">Filtros</CardTitle>
        </CardHeader>
        <CardContent>
          <EventFilters rules={rules} initialAfter={after !== undefined ? String(after) : ""} initialRule={rule ?? ""} />
          {!rulesResult.ok ? <p className="mt-2 text-xs text-muted-foreground">{rulesResult.error}</p> : null}
        </CardContent>
      </Card>

      {!eventsResult.ok ? <ErrorState message={eventsResult.error} code={eventsResult.code} /> : null}

      {eventsResult.ok ? (
        <Card>
          <CardHeader className="border-b">
            <div className="flex flex-wrap items-center justify-between gap-2">
              <div className="space-y-1">
                <CardTitle className="text-sm">{events.length} evento(s)</CardTitle>
                <CardDescription>
                  Cursor atual: <span className="font-mono">after={after ?? 0}</span> · próximo{" "}
                  <span className="font-mono">next_after={nextAfter}</span>
                </CardDescription>
              </div>
              <Link href={`/events?${advanceParams.toString()}`} className={buttonVariants({ variant: "outline", size: "sm" })}>
                Avançar cursor
                <ArrowRight aria-hidden />
              </Link>
            </div>
          </CardHeader>
          <CardContent>
            {events.length === 0 ? (
              <EmptyState
                title="Nenhum evento"
                description={
                  after
                    ? "Não há eventos após este cursor. Novos eventos aparecem aqui quando mensagens casarem com uma regra."
                    : "Nenhum evento registrado ainda. Crie regras para começar a capturar mensagens."
                }
                action={
                  <Link href="/rules" className={buttonVariants({ variant: "outline", size: "sm" })}>
                    Ver regras
                  </Link>
                }
              />
            ) : (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead className="w-16">seq</TableHead>
                    <TableHead>horário</TableHead>
                    <TableHead>chat</TableHead>
                    <TableHead>remetente</TableHead>
                    <TableHead>flags</TableHead>
                    <TableHead>conteúdo</TableHead>
                    <TableHead>mídia</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {events.map((event) => (
                    <TableRow key={event.seq}>
                      <TableCell className="font-mono text-xs tabular-nums">{event.seq}</TableCell>
                      <TableCell className="text-xs" title={formatDateTime(event.timestamp)}>
                        {formatDateTime(event.timestamp)}
                      </TableCell>
                      <TableCell className="max-w-40">
                        <p className="truncate text-xs">{event.chat_name || event.chat_jid}</p>
                        <p className="truncate font-mono text-[10px] text-muted-foreground">{event.chat_jid}</p>
                      </TableCell>
                      <TableCell className="max-w-36">
                        <p className="truncate text-xs">{event.sender_name || event.sender_jid || "—"}</p>
                      </TableCell>
                      <TableCell>
                        <div className="flex flex-wrap gap-1">
                          {event.is_group ? <Badge variant="secondary">grupo</Badge> : null}
                          {event.is_from_me ? <Badge variant="outline">from-me</Badge> : null}
                          {event.mentioned_me ? <Badge variant="outline">menção</Badge> : null}
                          {!event.is_group && !event.is_from_me && !event.mentioned_me ? (
                            <span className="text-xs text-muted-foreground">—</span>
                          ) : null}
                        </div>
                      </TableCell>
                      <TableCell className="max-w-72 whitespace-normal">
                        <p className="line-clamp-2 text-xs" title={event.content}>
                          {truncate(event.content || "(sem conteúdo)", 160)}
                        </p>
                      </TableCell>
                      <TableCell className="text-xs text-muted-foreground">{event.media_type ?? "—"}</TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )}
          </CardContent>
        </Card>
      ) : null}

      <Card className="mt-4">
        <CardHeader>
          <CardTitle className="text-sm">Ack de consumer</CardTitle>
          <CardDescription>
            Persiste o cursor do consumer no bridge. Use o mesmo <span className="font-mono">rule_id</span> e{" "}
            <span className="font-mono">seq</span> que você já processou.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <AckForm tenantId={TENANT_ID} rules={rules} defaultRule={rule ?? ""} defaultSeq={nextAfter} />
        </CardContent>
      </Card>
    </>
  );
}
