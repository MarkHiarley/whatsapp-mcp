import Link from "next/link";
import { connection } from "next/server";
import { MessagesSquare, Search, Users } from "lucide-react";

import { EmptyState } from "@/components/empty-state";
import { ErrorState } from "@/components/error-state";
import { PageHeader } from "@/components/page-header";
import { Avatar, AvatarFallback } from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
import { buttonVariants } from "@/components/ui/button";
import { Card, CardContent, CardHeader } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { loadChats } from "@/lib/api";
import { formatRelative, initials, phoneFromJid, truncate } from "@/lib/format";

export const instant = false;

export default async function ChatsPage({
  searchParams,
}: {
  searchParams: Promise<{ query?: string }>;
}) {
  await connection();

  const { query } = await searchParams;
  const result = await loadChats(query);

  return (
    <>
      <PageHeader
        title="Conversas"
        description="Conversas ordenadas pela última atividade. Busque por nome ou JID."
      />

      <Card className="mb-4">
        <CardHeader>
          <form action="/chats" method="get" className="flex flex-col gap-2 sm:flex-row">
            <div className="relative flex-1">
              <Search
                className="pointer-events-none absolute top-1/2 left-2.5 size-3.5 -translate-y-1/2 text-muted-foreground"
                aria-hidden
              />
              <Input
                type="search"
                name="query"
                defaultValue={query ?? ""}
                placeholder="Buscar por nome ou JID"
                className="pl-8"
                aria-label="Buscar chats"
              />
            </div>
            <div className="flex gap-2">
              <button type="submit" className={buttonVariants({ size: "sm" })}>
                Buscar
              </button>
              {query ? (
                <Link href="/chats" className={buttonVariants({ variant: "ghost", size: "sm" })}>
                  Limpar
                </Link>
              ) : null}
            </div>
          </form>
        </CardHeader>
      </Card>

      {!result.ok ? <ErrorState message={result.error} code={result.code} /> : null}

      {result.ok && result.data.length === 0 ? (
        <EmptyState
          title={query ? "Nenhum chat encontrado" : "Nenhum chat sincronizado"}
          description={
            query
              ? `Nada corresponde a “${query}”. Tente outro nome ou o JID completo.`
              : "As conversas aparecem depois que o bridge sincroniza a sessão do WhatsApp."
          }
          action={
            <Link href={query ? "/chats" : "/settings"} className={buttonVariants({ variant: "outline", size: "sm" })}>
              {query ? "Ver todos" : "Parear / QR"}
            </Link>
          }
        />
      ) : null}

      {result.ok && result.data.length > 0 ? (
        <Card>
          <CardContent className="divide-y">
            {result.data.map((chat) => (
              <Link
                key={chat.jid}
                href={`/chats/${encodeURIComponent(chat.jid)}`}
                className="-mx-2 flex items-center gap-3 rounded-lg px-2 py-3 transition-colors hover:bg-muted/60"
              >
                <Avatar>
                  <AvatarFallback>{initials(chat.name || phoneFromJid(chat.jid))}</AvatarFallback>
                </Avatar>
                <div className="min-w-0 flex-1">
                  <div className="flex items-center gap-2">
                    <p className="truncate text-sm font-medium">{chat.name || phoneFromJid(chat.jid)}</p>
                    {chat.is_group ? (
                      <Badge variant="secondary" className="gap-1">
                        <Users className="size-3" aria-hidden />
                        grupo
                      </Badge>
                    ) : null}
                  </div>
                  <p className="truncate text-xs text-muted-foreground">
                    {chat.last_message
                      ? `${chat.last_is_from_me ? "você: " : ""}${truncate(chat.last_message, 90)}`
                      : "sem mensagens"}
                  </p>
                  <p className="truncate font-mono text-[11px] text-muted-foreground/70">{chat.jid}</p>
                </div>
                <div className="flex shrink-0 flex-col items-end gap-1">
                  <span className="text-[11px] text-muted-foreground">{formatRelative(chat.last_message_time)}</span>
                  {chat.last_media_type ? (
                    <MessagesSquare className="size-3.5 text-muted-foreground" aria-hidden />
                  ) : null}
                </div>
              </Link>
            ))}
          </CardContent>
        </Card>
      ) : null}
    </>
  );
}
