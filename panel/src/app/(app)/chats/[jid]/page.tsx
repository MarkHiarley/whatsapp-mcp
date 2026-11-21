import Link from "next/link";
import { connection } from "next/server";
import { ChevronLeft, Users } from "lucide-react";

import { MessageMedia } from "@/components/chats/message-media";
import { SendMessageForm } from "@/components/chats/send-message-form";
import { EmptyState } from "@/components/empty-state";
import { ErrorState } from "@/components/error-state";
import { StatusBadge } from "@/components/status-badge";
import { Avatar, AvatarFallback } from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
import { buttonVariants } from "@/components/ui/button";
import { Card, CardContent, CardFooter, CardHeader, CardTitle } from "@/components/ui/card";
import { loadChats, loadMessages, loadStatus } from "@/lib/api";
import { formatDateTime, formatTime, initials, phoneFromJid } from "@/lib/format";
import { requireTenantContext } from "@/lib/session";
import { cn } from "@/lib/utils";

export const instant = false;

function safeDecode(value: string): string {
  try {
    return decodeURIComponent(value);
  } catch {
    return value;
  }
}

export default async function ChatDetailPage({
  params,
  searchParams,
}: {
  params: Promise<{ jid: string }>;
  searchParams: Promise<{ before?: string }>;
}) {
  await connection();

  const { tenantId } = await requireTenantContext();
  const { jid: rawJid } = await params;
  const jid = safeDecode(rawJid);

  const { before } = await searchParams;
  const [messagesResult, statusResult, chatsResult] = await Promise.all([
    loadMessages(jid, before),
    loadStatus(),
    loadChats(jid, 10),
  ]);

  const chat = chatsResult.ok ? chatsResult.data.find((item) => item.jid === jid) : undefined;
  const title = chat?.name || phoneFromJid(jid);
  const connected = statusResult.ok && statusResult.data.status === "connected";
  const messages = messagesResult.ok ? messagesResult.data.messages : [];
  const oldest = messages[0];

  return (
    <>
      <div className="mb-4 flex items-center justify-between gap-2">
        <BackLink />
        {statusResult.ok ? <StatusBadge status={statusResult.data.status} /> : null}
      </div>

      <div className="mb-4 flex items-center gap-3">
        <Avatar size="lg">
          <AvatarFallback>{initials(title)}</AvatarFallback>
        </Avatar>
        <div className="min-w-0">
          <h1 className="flex items-center gap-2 font-heading text-lg font-semibold">
            <span className="truncate">{title}</span>
            {chat?.is_group ? (
              <Badge variant="secondary" className="gap-1">
                <Users className="size-3" aria-hidden />
                grupo
              </Badge>
            ) : null}
          </h1>
          <p className="truncate font-mono text-xs text-muted-foreground">{jid}</p>
        </div>
      </div>

      {!messagesResult.ok ? <ErrorState message={messagesResult.error} code={messagesResult.code} /> : null}

      {messagesResult.ok ? (
        <Card>
          <CardHeader className="border-b">
            <CardTitle className="flex flex-wrap items-center justify-between gap-2 text-sm font-medium">
              <span>
                {messages.length} mensagem{messages.length === 1 ? "" : "s"}
                {before ? " (janela anterior)" : ""}
              </span>
              {before ? (
                <Link href={`/chats/${encodeURIComponent(jid)}`} className={buttonVariants({ variant: "ghost", size: "xs" })}>
                  Ir para as mais recentes
                </Link>
              ) : null}
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            {messages.length === 0 ? (
              <EmptyState
                title={before ? "Nada mais para carregar" : "Nenhuma mensagem"}
                description={
                  before
                    ? "Não há mensagens anteriores a este ponto."
                    : "As mensagens aparecem aqui quando o bridge sincroniza esta conversa."
                }
              />
            ) : (
              <>
                {oldest ? (
                  <div className="flex justify-center pb-2">
                    <Link
                      href={`/chats/${encodeURIComponent(jid)}?before=${encodeURIComponent(oldest.timestamp)}`}
                      className={buttonVariants({ variant: "outline", size: "xs" })}
                    >
                      Carregar mais antigas
                    </Link>
                  </div>
                ) : null}
                {messages.map((message) => (
                  <div
                    key={message.id}
                    className={cn("flex", message.is_from_me ? "justify-end" : "justify-start")}
                  >
                    <div
                      className={cn(
                        "max-w-[85%] min-w-0 rounded-lg px-3 py-2 text-sm shadow-xs sm:max-w-[70%]",
                        message.is_from_me
                          ? "rounded-br-sm bg-primary text-primary-foreground"
                          : "rounded-bl-sm bg-muted text-foreground",
                      )}
                    >
                      {!message.is_from_me ? (
                        <p className="mb-0.5 truncate text-[11px] font-medium opacity-70">{message.sender}</p>
                      ) : null}

                      <MessageMedia message={message} jid={jid} tenantId={tenantId} />

                      {message.content ? (
                        <p className="break-words whitespace-pre-wrap">{message.content}</p>
                      ) : message.media_type ? null : (
                        <p className="text-xs italic opacity-70">(sem conteúdo)</p>
                      )}

                      <div className="mt-1 flex items-center justify-end gap-1">
                        <span className="text-[10px] opacity-60" title={formatDateTime(message.timestamp)}>
                          {formatTime(message.timestamp)}
                        </span>
                      </div>
                    </div>
                  </div>
                ))}
              </>
            )}
          </CardContent>
          <CardFooter className="sticky bottom-0 z-10 flex-col items-stretch gap-2 bg-card">
            {connected ? null : (
              <p className="text-xs text-muted-foreground">
                Sessão não está conectada — o envio fica desabilitado até o pareamento terminar.
              </p>
            )}
            <SendMessageForm tenantId={tenantId} jid={jid} disabled={!connected} />
          </CardFooter>
        </Card>
      ) : null}
    </>
  );
}

function BackLink() {
  return (
    <Link href="/chats" className={buttonVariants({ variant: "ghost", size: "sm" })}>
      <ChevronLeft aria-hidden />
      Conversas
    </Link>
  );
}
