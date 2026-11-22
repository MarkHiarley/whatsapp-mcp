import os
from dataclasses import fields, is_dataclass
from datetime import datetime
from pathlib import Path
from typing import List, Optional
from typing_extensions import NotRequired, TypedDict
from mcp.server.fastmcp import FastMCP
from mcp.types import ToolAnnotations

import approvals
import audit
import remote
import whatsapp
from whatsapp import (
    search_contacts as whatsapp_search_contacts,
    list_messages as whatsapp_list_messages,
    list_chats as whatsapp_list_chats,
    get_chat as whatsapp_get_chat,
    get_direct_chat_by_contact as whatsapp_get_direct_chat_by_contact,
    get_contact_chats as whatsapp_get_contact_chats,
    get_last_interaction as whatsapp_get_last_interaction,
    get_message_context as whatsapp_get_message_context,
    send_message as whatsapp_send_message,
    send_file as whatsapp_send_file,
    send_audio_message as whatsapp_audio_voice_message,
    download_media as whatsapp_download_media,
    list_events as whatsapp_list_events,
    wait_for_events as whatsapp_wait_for_events,
    list_rules as whatsapp_list_rules,
    ack_events as whatsapp_ack_events,
    _localize_dt
)


class MessageDTO(TypedDict):
    id: str
    timestamp: str
    sender: str
    sender_name: str
    content: str
    is_from_me: bool
    chat_jid: str
    chat_name: Optional[str]
    media_type: Optional[str]


class ChatDTO(TypedDict):
    jid: str
    name: Optional[str]
    last_message_time: Optional[str]
    last_message: Optional[str]
    last_sender: Optional[str]
    last_is_from_me: Optional[bool]


class ContactDTO(TypedDict):
    phone_number: str
    name: Optional[str]
    jid: str


class MessageContextDTO(TypedDict):
    message: MessageDTO
    before: List[MessageDTO]
    after: List[MessageDTO]


class ActionResult(TypedDict):
    success: bool
    message: str


class MediaResult(ActionResult):
    file_path: NotRequired[str]


class DraftDTO(TypedDict):
    approval_id: str
    kind: str
    recipient: str
    preview: str
    expires_at: str


class EventDTO(TypedDict):
    seq: int
    message_id: str
    chat_jid: str
    chat_name: NotRequired[str]
    sender_jid: NotRequired[str]
    sender_name: NotRequired[str]
    is_group: bool
    is_from_me: bool
    mentioned_me: bool
    content: str
    media_type: NotRequired[str]
    timestamp: str
    created_at: str


class RuleMatchDTO(TypedDict, total=False):
    chat_jids: List[str]
    sender_jids: List[str]
    is_group: bool
    is_from_me: bool
    mentioned_me: bool
    text_contains: List[str]
    text_regex: str


class RuleDTO(TypedDict):
    id: str
    name: str
    match: RuleMatchDTO
    enabled: bool
    created_at: str


def _contract(value):
    if isinstance(value, datetime):
        return _localize_dt(value).isoformat()
    if is_dataclass(value):
        return {field.name: _contract(getattr(value, field.name)) for field in fields(value)}
    if isinstance(value, list):
        return [_contract(item) for item in value]
    if isinstance(value, dict):
        return {key: _contract(item) for key, item in value.items()}
    return value


# Initialize FastMCP server. HTTP mode requires bearer tokens and binds each
# request to one tenant; STDIO keeps the local environment credentials.
REMOTE = remote.transport() == "streamable-http"
if REMOTE:
    INTROSPECT_URL = os.getenv("WHATSAPP_MCP_INTROSPECT_URL", "").strip()
    INTROSPECT_SECRET = os.getenv("WHATSAPP_MCP_INTROSPECT_SECRET", "").strip()
    if INTROSPECT_URL:
        if not INTROSPECT_SECRET:
            raise RuntimeError("WHATSAPP_MCP_INTROSPECT_SECRET is required with WHATSAPP_MCP_INTROSPECT_URL")
        verifier = remote.IntrospectionTokenStore(INTROSPECT_URL, INTROSPECT_SECRET)
    else:
        REMOTE_TOKENS = remote.load_tokens()
        if not REMOTE_TOKENS:
            raise RuntimeError("HTTP mode requires WHATSAPP_MCP_INTROSPECT_URL or a token file")
        verifier = remote.TokenStore(REMOTE_TOKENS)
    mcp = remote.ScopedFastMCP(
        "whatsapp",
        host=remote.http_host(),
        port=remote.http_port(),
        streamable_http_path=remote.http_path(),
        token_verifier=verifier,
        auth=remote.auth_settings(),
        transport_security=remote.transport_security(),
    )
else:
    mcp = FastMCP("whatsapp")

READ_ONLY = os.getenv("WHATSAPP_MCP_READ_ONLY", "").lower() in {"1", "true", "yes", "on"}
ALLOW_DIRECT_SEND = os.getenv("WHATSAPP_MCP_ALLOW_DIRECT_SEND", "").lower() in {"1", "true", "yes", "on"}

READ_ANNOTATIONS = ToolAnnotations(readOnlyHint=True, openWorldHint=False)
DRAFT_ANNOTATIONS = ToolAnnotations(readOnlyHint=False, destructiveHint=False, idempotentHint=False)
DESTRUCTIVE_ANNOTATIONS = ToolAnnotations(readOnlyHint=False, destructiveHint=True, idempotentHint=False)
CURSOR_ANNOTATIONS = ToolAnnotations(readOnlyHint=False, destructiveHint=False, idempotentHint=True)


def read_tool(fn):
    return mcp.tool(annotations=READ_ANNOTATIONS)(audit.audited(fn))

@read_tool
def search_contacts(query: str) -> List[ContactDTO]:
    """Search WhatsApp contacts by name or phone number.
    
    Args:
        query: Search term to match against contact names or phone numbers
    """
    contacts = whatsapp_search_contacts(query)
    return _contract(contacts)

@read_tool
def list_messages(
    after: Optional[str] = None,
    before: Optional[str] = None,
    sender_phone_number: Optional[str] = None,
    chat_jid: Optional[str] = None,
    query: Optional[str] = None,
    limit: int = 20,
    page: int = 0,
    include_context: bool = True,
    context_before: int = 1,
    context_after: int = 1
) -> List[MessageDTO]:
    """Get WhatsApp messages matching specified criteria with optional context.
    
    Args:
        after: Optional ISO-8601 formatted string to only return messages after this date
        before: Optional ISO-8601 formatted string to only return messages before this date
        sender_phone_number: Optional phone number to filter messages by sender
        chat_jid: Optional chat JID to filter messages by chat
        query: Optional search term to filter messages by content
        limit: Maximum number of messages to return (default 20)
        page: Page number for pagination (default 0)
        include_context: Whether to include messages before and after matches (default True)
        context_before: Number of messages to include before each match (default 1)
        context_after: Number of messages to include after each match (default 1)
    """
    messages = whatsapp_list_messages(
        after=after,
        before=before,
        sender_phone_number=sender_phone_number,
        chat_jid=chat_jid,
        query=query,
        limit=limit,
        page=page,
        include_context=include_context,
        context_before=context_before,
        context_after=context_after
    )
    return _contract(messages)

@read_tool
def list_chats(
    query: Optional[str] = None,
    limit: int = 20,
    page: int = 0,
    include_last_message: bool = True,
    sort_by: str = "last_active"
) -> List[ChatDTO]:
    """Get WhatsApp chats matching specified criteria.
    
    Args:
        query: Optional search term to filter chats by name or JID
        limit: Maximum number of chats to return (default 20)
        page: Page number for pagination (default 0)
        include_last_message: Whether to include the last message in each chat (default True)
        sort_by: Field to sort results by, either "last_active" or "name" (default "last_active")
    """
    chats = whatsapp_list_chats(
        query=query,
        limit=limit,
        page=page,
        include_last_message=include_last_message,
        sort_by=sort_by
    )
    return _contract(chats)

@read_tool
def get_chat(chat_jid: str, include_last_message: bool = True) -> Optional[ChatDTO]:
    """Get WhatsApp chat metadata by JID.
    
    Args:
        chat_jid: The JID of the chat to retrieve
        include_last_message: Whether to include the last message (default True)
    """
    chat = whatsapp_get_chat(chat_jid, include_last_message)
    return _contract(chat)

@read_tool
def get_direct_chat_by_contact(sender_phone_number: str) -> Optional[ChatDTO]:
    """Get WhatsApp chat metadata by sender phone number.
    
    Args:
        sender_phone_number: The phone number to search for
    """
    chat = whatsapp_get_direct_chat_by_contact(sender_phone_number)
    return _contract(chat)

@read_tool
def get_contact_chats(jid: str, limit: int = 20, page: int = 0) -> List[ChatDTO]:
    """Get all WhatsApp chats involving the contact.
    
    Args:
        jid: The contact's JID to search for
        limit: Maximum number of chats to return (default 20)
        page: Page number for pagination (default 0)
    """
    chats = whatsapp_get_contact_chats(jid, limit, page)
    return _contract(chats)

@read_tool
def get_last_interaction(jid: str) -> Optional[MessageDTO]:
    """Get most recent WhatsApp message involving the contact.
    
    Args:
        jid: The JID of the contact to search for
    """
    message = whatsapp_get_last_interaction(jid)
    return _contract(message)

@read_tool
def get_message_context(
    message_id: str,
    before: int = 5,
    after: int = 5
) -> MessageContextDTO:
    """Get context around a specific WhatsApp message.
    
    Args:
        message_id: The ID of the message to get context for
        before: Number of messages to include before the target message (default 5)
        after: Number of messages to include after the target message (default 5)
    """
    context = whatsapp_get_message_context(message_id, before, after)
    return _contract(context)

def send_message(
    recipient: str,
    message: str
) -> ActionResult:
    """Send a WhatsApp message to a person or group. For group chats use the JID.

    Args:
        recipient: The recipient - either a phone number with country code but no + or other symbols,
                 or a JID (e.g., "123456789@s.whatsapp.net" or a group JID like "123456789@g.us")
        message: The message text to send
    
    Returns:
        A dictionary containing success status and a status message
    """
    # Validate input
    if not recipient:
        return {
            "success": False,
            "message": "Recipient must be provided"
        }
    
    # Call the whatsapp_send_message function with the unified recipient parameter
    success, status_message = whatsapp_send_message(recipient, message)
    return {
        "success": success,
        "message": status_message
    }

def send_file(recipient: str, media_path: str) -> ActionResult:
    """Send a file such as a picture, raw audio, video or document via WhatsApp to the specified recipient. For group messages use the JID.
    
    Args:
        recipient: The recipient - either a phone number with country code but no + or other symbols,
                 or a JID (e.g., "123456789@s.whatsapp.net" or a group JID like "123456789@g.us")
        media_path: The absolute path to the media file to send (image, video, document)
    
    Returns:
        A dictionary containing success status and a status message
    """
    
    try:
        media_path = str(approvals.resolve_outbox_file(media_path))
    except ValueError as exc:
        return {"success": False, "message": str(exc)}

    # Call the whatsapp_send_file function
    success, status_message = whatsapp_send_file(recipient, media_path)
    return {
        "success": success,
        "message": status_message
    }

def send_audio_message(recipient: str, media_path: str) -> ActionResult:
    """Send any audio file as a WhatsApp audio message to the specified recipient. For group messages use the JID. If it errors due to ffmpeg not being installed, use send_file instead.
    
    Args:
        recipient: The recipient - either a phone number with country code but no + or other symbols,
                 or a JID (e.g., "123456789@s.whatsapp.net" or a group JID like "123456789@g.us")
        media_path: The absolute path to the audio file to send (will be converted to Opus .ogg if it's not a .ogg file)
    
    Returns:
        A dictionary containing success status and a status message
    """
    try:
        media_path = str(approvals.resolve_outbox_file(media_path))
    except ValueError as exc:
        return {"success": False, "message": str(exc)}

    success, status_message = whatsapp_audio_voice_message(recipient, media_path)
    return {
        "success": success,
        "message": status_message
    }

def _tenant_id() -> str:
    return whatsapp.current_tenant()


def _draft_dto(approval: dict, preview: str) -> DraftDTO:
    return {
        "approval_id": approval["approval_id"],
        "kind": approval["kind"],
        "recipient": approval["recipient"],
        "preview": preview,
        "expires_at": approvals.iso_from_epoch(approval["expires_at"]),
    }


def draft_message(recipient: str, message: str) -> DraftDTO:
    """Create a message draft that a human must approve before it is sent.

    This tool never sends anything. It returns an ``approval_id`` and the exact
    preview. Call ``confirm_send`` only after the human has reviewed and approved
    this draft.
    """
    if not recipient or not recipient.strip():
        raise ValueError("recipient must be provided")
    if message is None or not str(message).strip():
        raise ValueError("message must be provided")
    approval = approvals.create(
        tenant_id=_tenant_id(),
        kind="message",
        recipient=recipient.strip(),
        text=str(message),
    )
    return _draft_dto(approval, str(message))


def draft_file(recipient: str, media_path: str) -> DraftDTO:
    """Create a file draft confined to the outbox; sends nothing.

    The file is hashed now and re-checked at confirmation time.
    """
    if not recipient or not recipient.strip():
        raise ValueError("recipient must be provided")
    path = approvals.resolve_outbox_file(media_path)
    digest = approvals.hash_file(path)
    approval = approvals.create(
        tenant_id=_tenant_id(),
        kind="file",
        recipient=recipient.strip(),
        file_path=path,
        file_sha256=digest,
    )
    preview = f"{path.name} ({path.stat().st_size} bytes, sha256:{digest[:16]})"
    return _draft_dto(approval, preview)


def draft_audio_message(recipient: str, media_path: str) -> DraftDTO:
    """Create an audio draft confined to the outbox; sends nothing."""
    if not recipient or not recipient.strip():
        raise ValueError("recipient must be provided")
    path = approvals.resolve_outbox_file(media_path)
    digest = approvals.hash_file(path)
    approval = approvals.create(
        tenant_id=_tenant_id(),
        kind="audio",
        recipient=recipient.strip(),
        file_path=path,
        file_sha256=digest,
    )
    preview = f"{path.name} ({path.stat().st_size} bytes, sha256:{digest[:16]})"
    return _draft_dto(approval, preview)


def confirm_send(approval_id: str, confirm: bool = False) -> ActionResult:
    """Send a previously drafted action after explicit human approval.

    ``confirm=true`` is required, and the approval must exist, belong to this
    tenant, be unused and be younger than the configured TTL. Files are re-hashed
    immediately before sending.
    """
    if not confirm:
        return {
            "success": False,
            "message": "Human approval required: review the draft, then call confirm_send with confirm=true.",
        }
    approval = approvals.consume(tenant_id=_tenant_id(), approval_id=approval_id)
    if approval is None:
        return {
            "success": False,
            "message": "Approval not found, expired or already used. Create a new draft.",
        }

    kind = approval["kind"]
    recipient = approval["recipient"]
    if kind == "message":
        success, status_message = whatsapp_send_message(recipient, approval.get("text") or "")
    elif kind in {"file", "audio"}:
        if not approvals.file_still_matches(approval):
            return {
                "success": False,
                "message": "The approved file changed or left the outbox; create a new draft.",
            }
        if kind == "file":
            success, status_message = whatsapp_send_file(recipient, approval["file_path"])
        else:
            success, status_message = whatsapp_audio_voice_message(recipient, approval["file_path"])
    else:
        return {"success": False, "message": f"Unsupported approval kind: {kind}"}

    return {"success": success, "message": status_message}


@read_tool
def download_media(message_id: str, chat_jid: str) -> MediaResult:
    """Download media from a WhatsApp message and get the local file path.
    
    Args:
        message_id: The ID of the message containing the media
        chat_jid: The JID of the chat containing the message
    
    Returns:
        A dictionary containing success status, a status message, and the file path if successful
    """
    file_path = whatsapp_download_media(message_id, chat_jid)
    
    if file_path:
        return {
            "success": True,
            "message": "Media downloaded successfully",
            "file_path": file_path
        }
    else:
        return {
            "success": False,
            "message": "Failed to download media"
        }

@read_tool
def list_events(after: int = 0, rule: Optional[str] = None, limit: int = 50) -> List[EventDTO]:
    """List durable WhatsApp events for the tenant of this server process (WHATSAPP_TENANT_ID).

    Events come in ascending `seq` order. `after` is the cursor: pass the last `seq` you processed
    to get only newer events. Use `list_rules` to find rule ids.

    Args:
        after: Cursor. Returns events with seq greater than this value (default 0 = from the start)
        rule: Optional rule id; when set, only events matched by that rule are returned
        limit: Maximum number of events to return, 1 to 500 (default 50)
    """
    events = whatsapp_list_events(after=after, rule=rule, limit=limit)
    return _contract(events)

@read_tool
def wait_for_events(after: int = 0, timeout_seconds: int = 30, rule: Optional[str] = None) -> List[EventDTO]:
    """Wait for new WhatsApp events for the tenant of this server process (WHATSAPP_TENANT_ID).

    Returns as soon as the first non-empty batch arrives, or an empty list when `timeout_seconds`
    expires. `after` is the cursor: pass the last `seq` you processed.

    Args:
        after: Cursor. Only events with seq greater than this value are returned (default 0)
        timeout_seconds: Maximum time to wait in seconds (default 30)
        rule: Optional rule id to filter events
    """
    events = whatsapp_wait_for_events(after=after, timeout=timeout_seconds, rule=rule)
    return _contract(events)

@read_tool
def list_rules() -> List[RuleDTO]:
    """List the event rules of the tenant of this server process (WHATSAPP_TENANT_ID).

    Each rule has an `id` to use with `list_events`, `wait_for_events` and `ack_events`.
    """
    rules = whatsapp_list_rules()
    return _contract(rules)

def ack_events(consumer_id: str, rule_id: str, seq: int) -> ActionResult:
    """Save the consumer cursor for a rule of the tenant of this server process (WHATSAPP_TENANT_ID).

    Delivery is at-least-once: after a restart, events after the saved `seq` may come again.

    Args:
        consumer_id: Consumer name (letters, digits, '_' or '-', up to 64 characters)
        rule_id: The rule id the consumer is reading
        seq: The last `seq` fully processed for this rule (non-negative)
    """
    success, status_message = whatsapp_ack_events(consumer_id, rule_id, seq)
    return {
        "success": success,
        "message": status_message
    }

if ALLOW_DIRECT_SEND and not READ_ONLY:
    # Opt-in legacy tools: they send immediately and bypass the approval flow.
    mcp.tool(annotations=DESTRUCTIVE_ANNOTATIONS)(audit.audited(send_message, required_scope="write"))
    mcp.tool(annotations=DESTRUCTIVE_ANNOTATIONS)(audit.audited(send_file, required_scope="write"))
    mcp.tool(annotations=DESTRUCTIVE_ANNOTATIONS)(audit.audited(send_audio_message, required_scope="write"))

if not READ_ONLY:
    mcp.tool(annotations=DRAFT_ANNOTATIONS)(audit.audited(draft_message, required_scope="write"))
    mcp.tool(annotations=DRAFT_ANNOTATIONS)(audit.audited(draft_file, required_scope="write"))
    mcp.tool(annotations=DRAFT_ANNOTATIONS)(audit.audited(draft_audio_message, required_scope="write"))
    mcp.tool(annotations=DESTRUCTIVE_ANNOTATIONS)(audit.audited(confirm_send, required_scope="write"))
    mcp.tool(annotations=CURSOR_ANNOTATIONS)(audit.audited(ack_events, required_scope="write"))

if __name__ == "__main__":
    mcp.run(transport="streamable-http" if REMOTE else "stdio")
