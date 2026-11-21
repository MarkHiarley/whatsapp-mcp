import type { Schema } from "@/lib/api";

type RuleMatch = Schema["RuleMatch"];

function join(values?: string[]): string | null {
  if (!values || values.length === 0) {
    return null;
  }
  return values.length <= 2 ? values.join(", ") : `${values.slice(0, 2).join(", ")} +${values.length - 2}`;
}

/** Short human summary of a rule match, without dumping the whole payload. */
export function summarizeMatch(match: RuleMatch): string {
  const parts: string[] = [];
  const chats = join(match.chat_jids);
  if (chats) {
    parts.push(`chat: ${chats}`);
  }
  const senders = join(match.sender_jids);
  if (senders) {
    parts.push(`remetente: ${senders}`);
  }
  const contains = join(match.text_contains);
  if (contains) {
    parts.push(`contém: ${contains}`);
  }
  if (match.text_regex) {
    parts.push(`regex: ${match.text_regex}`);
  }
  if (match.is_group !== undefined) {
    parts.push(match.is_group ? "grupo" : "não-grupo");
  }
  if (match.is_from_me !== undefined) {
    parts.push(match.is_from_me ? "enviada por mim" : "recebida");
  }
  if (match.mentioned_me) {
    parts.push("menciona mim");
  }
  return parts.length > 0 ? parts.join(" · ") : "sem filtros";
}
