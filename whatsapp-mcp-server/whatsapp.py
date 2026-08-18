import sqlite3
from datetime import datetime, timezone, timedelta
import time

# Detect local timezone offset for display
_local_tz = timezone(timedelta(seconds=-time.timezone if time.timezone < 0 else -time.timezone)) if hasattr(time, 'timezone') else timezone.utc
_tz_name = time.tzname[0] if hasattr(time, 'tzname') and time.tzname else 'UTC'

def _localize_dt(dt):
    if dt is None: return dt
    if dt.tzinfo is None:
        return dt.replace(tzinfo=timezone.utc).astimezone(_local_tz)
    return dt.astimezone(_local_tz)

from dataclasses import dataclass
from typing import Optional, List, Tuple
import os.path
import requests
import json
import audio

MESSAGES_DB_PATH = os.path.join(os.path.dirname(os.path.abspath(__file__)), '..', 'whatsapp-bridge', 'store', 'messages.db')
WHATSAPP_API_BASE_URL = "http://localhost:8080/api/v1"
WHATSAPP_API_TOKEN = os.environ.get("WHATSAPP_API_TOKEN", "")


def _api_headers():
    if not WHATSAPP_API_TOKEN:
        raise RuntimeError("WHATSAPP_API_TOKEN is required")
    return {"Authorization": f"Bearer {WHATSAPP_API_TOKEN}"}


def _bridge_response(response):
    payload = response.json()
    if payload.get("success"):
        return True, payload.get("data") or {}
    error = payload.get("error") or {}
    return False, {"message": error.get("message", f"HTTP {response.status_code}")}


def _connect():
    return sqlite3.connect(MESSAGES_DB_PATH)


def _contact_identity(conn, identifier):
    user = identifier.split('@')[0]
    try:
        row = conn.execute("""
            SELECT phone, lid,
                   COALESCE(NULLIF(display_name, ''), NULLIF(push_name, ''), phone)
            FROM contacts
            WHERE phone = ? OR lid = ?
            LIMIT 1
        """, (user, user)).fetchone()
    except sqlite3.OperationalError:
        row = None
    return row


def _identity_aliases(conn, identifier):
    if identifier.endswith('@g.us'):
        return [identifier]
    identity = _contact_identity(conn, identifier)
    if not identity:
        user = identifier.split('@')[0]
        return list(dict.fromkeys((identifier, user, f"{user}@s.whatsapp.net", f"{user}@lid")))
    phone, lid, _ = identity
    return list(dict.fromkeys(filter(None, (
        phone, lid, f"{phone}@s.whatsapp.net", f"{lid}@lid" if lid else None,
    ))))


@dataclass
class Message:
    timestamp: datetime
    sender: str
    content: str
    is_from_me: bool
    chat_jid: str
    id: str
    chat_name: Optional[str] = None
    media_type: Optional[str] = None
    sender_name: Optional[str] = None

@dataclass
class Chat:
    jid: str
    name: Optional[str]
    last_message_time: Optional[datetime]
    last_message: Optional[str] = None
    last_sender: Optional[str] = None
    last_is_from_me: Optional[bool] = None

    @property
    def is_group(self) -> bool:
        """Determine if chat is a group based on JID pattern."""
        return self.jid.endswith("@g.us")

@dataclass
class Contact:
    phone_number: str
    name: Optional[str]
    jid: str

@dataclass
class MessageContext:
    message: Message
    before: List[Message]
    after: List[Message]


def _chat_from_row(conn, row):
    jid, name = row[0], row[1]
    identity = None if jid.endswith(('@g.us', '@broadcast', '@newsletter')) else _contact_identity(conn, jid)
    if identity:
        jid = f"{identity[0]}@s.whatsapp.net"
        if identity[2] != identity[0]:
            name = identity[2]
    return Chat(
        jid=jid,
        name=name,
        last_message_time=datetime.fromisoformat(row[2]) if row[2] else None,
        last_message=row[3],
        last_sender=row[4],
        last_is_from_me=row[5],
    )


def get_sender_name(sender_jid: str) -> str:
    try:
        conn = _connect()
        identity = _contact_identity(conn, sender_jid)
        if identity and identity[2] != identity[0]:
            return identity[2]

        aliases = _identity_aliases(conn, sender_jid)
        placeholders = ",".join("?" * len(aliases))
        result = conn.execute(
            f"SELECT name FROM chats WHERE jid IN ({placeholders}) AND name != '' LIMIT 1",
            aliases,
        ).fetchone()
        return result[0] if result else sender_jid
    except sqlite3.Error as e:
        print(f"Database error while getting sender name: {e}")
        return sender_jid
    finally:
        if 'conn' in locals():
            conn.close()

def format_message(message: Message, show_chat_info: bool = True) -> str:
    """Print a single message with consistent formatting."""
    output = ""
    ts = _localize_dt(message.timestamp)
    ts_str = ts.strftime("%Y-%m-%d %H:%M:%S") + " " + _tz_name
    
    if show_chat_info and message.chat_name:
        output += f"[{ts_str}] Chat: {message.chat_name} "
    else:
        output += f"[{ts_str}] "
        
    content_prefix = ""
    if hasattr(message, 'media_type') and message.media_type:
        content_prefix = f"[{message.media_type} - Message ID: {message.id} - Chat JID: {message.chat_jid}] "
    
    try:
        sender_name = getattr(message, "sender_name", None) or (get_sender_name(message.sender) if not message.is_from_me else "Me")
        output += f"From: {sender_name}: {content_prefix}{message.content}\n"
    except Exception as e:
        print(f"Error formatting message: {e}")
    return output

def format_messages_list(messages: List[Message], show_chat_info: bool = True) -> str:
    output = ""
    if not messages:
        output += "No messages to display."
        return output
    
    for message in messages:
        output += format_message(message, show_chat_info)
    return output

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
) -> List[Message]:
    """Get messages matching the specified criteria with optional context."""
    try:
        conn = _connect()
        cursor = conn.cursor()
        
        # Build base query
        query_parts = ["SELECT messages.timestamp, messages.sender, chats.name, messages.content, messages.is_from_me, chats.jid, messages.id, messages.media_type FROM messages"]
        query_parts.append("JOIN chats ON messages.chat_jid = chats.jid")
        where_clauses = []
        params = []
        
        # Add filters
        if after:
            try:
                after = datetime.fromisoformat(after)
            except ValueError:
                raise ValueError(f"Invalid date format for 'after': {after}. Please use ISO-8601 format.")
            
            where_clauses.append("messages.timestamp > ?")
            params.append(after)

        if before:
            try:
                before = datetime.fromisoformat(before)
            except ValueError:
                raise ValueError(f"Invalid date format for 'before': {before}. Please use ISO-8601 format.")
            
            where_clauses.append("messages.timestamp < ?")
            params.append(before)

        if sender_phone_number:
            aliases = _identity_aliases(conn, sender_phone_number)
            where_clauses.append(f"messages.sender IN ({','.join('?' * len(aliases))})")
            params.extend(aliases)

        if chat_jid:
            aliases = _identity_aliases(conn, chat_jid)
            where_clauses.append(f"messages.chat_jid IN ({','.join('?' * len(aliases))})")
            params.extend(aliases)
            
        if query:
            where_clauses.append("LOWER(messages.content) LIKE LOWER(?)")
            params.append(f"%{query}%")
            
        if where_clauses:
            query_parts.append("WHERE " + " AND ".join(where_clauses))
            
        # Add pagination
        offset = page * limit
        query_parts.append("ORDER BY messages.timestamp DESC")
        query_parts.append("LIMIT ? OFFSET ?")
        params.extend([limit, offset])
        
        cursor.execute(" ".join(query_parts), tuple(params))
        messages = cursor.fetchall()
        
        result = []
        for msg in messages:
            message = Message(
                timestamp=datetime.fromisoformat(msg[0]),
                sender=msg[1],
                chat_name=msg[2],
                content=msg[3],
                is_from_me=msg[4],
                chat_jid=msg[5],
                id=msg[6],
                media_type=msg[7],
                sender_name="Me" if msg[4] else get_sender_name(msg[1])
            )
            result.append(message)
            
        if include_context and result:
            # Add context for each message
            messages_with_context = []
            for msg in result:
                context = get_message_context(msg.id, context_before, context_after)
                messages_with_context.extend(context.before)
                messages_with_context.append(context.message)
                messages_with_context.extend(context.after)
            
            return messages_with_context

        return result
        
    except sqlite3.Error as e:
        print(f"Database error: {e}")
        return []
    finally:
        if 'conn' in locals():
            conn.close()


def get_message_context(
    message_id: str,
    before: int = 5,
    after: int = 5
) -> MessageContext:
    """Get context around a specific message."""
    try:
        conn = sqlite3.connect(MESSAGES_DB_PATH)
        cursor = conn.cursor()
        
        # Get the target message first
        cursor.execute("""
            SELECT messages.timestamp, messages.sender, chats.name, messages.content, messages.is_from_me, chats.jid, messages.id, messages.chat_jid, messages.media_type
            FROM messages
            JOIN chats ON messages.chat_jid = chats.jid
            WHERE messages.id = ?
        """, (message_id,))
        msg_data = cursor.fetchone()
        
        if not msg_data:
            raise ValueError(f"Message with ID {message_id} not found")
            
        target_message = Message(
            timestamp=datetime.fromisoformat(msg_data[0]),
            sender=msg_data[1],
            chat_name=msg_data[2],
            content=msg_data[3],
            is_from_me=msg_data[4],
            chat_jid=msg_data[5],
            id=msg_data[6],
            media_type=msg_data[8],
            sender_name="Me" if msg_data[4] else get_sender_name(msg_data[1])
        )
        
        # Get messages before, including the contact's phone/LID alias.
        chat_aliases = _identity_aliases(conn, msg_data[7])
        placeholders = ",".join("?" * len(chat_aliases))
        cursor.execute(f"""
            SELECT messages.timestamp, messages.sender, chats.name, messages.content, messages.is_from_me, chats.jid, messages.id, messages.media_type
            FROM messages
            JOIN chats ON messages.chat_jid = chats.jid
            WHERE messages.chat_jid IN ({placeholders}) AND messages.timestamp < ?
            ORDER BY messages.timestamp DESC
            LIMIT ?
        """, (*chat_aliases, msg_data[0], before))
        
        before_messages = []
        for msg in cursor.fetchall():
            before_messages.append(Message(
                timestamp=datetime.fromisoformat(msg[0]),
                sender=msg[1],
                chat_name=msg[2],
                content=msg[3],
                is_from_me=msg[4],
                chat_jid=msg[5],
                id=msg[6],
                media_type=msg[7],
                sender_name="Me" if msg[4] else get_sender_name(msg[1])
            ))

        
        # Get messages after
        cursor.execute(f"""
            SELECT messages.timestamp, messages.sender, chats.name, messages.content, messages.is_from_me, chats.jid, messages.id, messages.media_type
            FROM messages
            JOIN chats ON messages.chat_jid = chats.jid
            WHERE messages.chat_jid IN ({placeholders}) AND messages.timestamp > ?
            ORDER BY messages.timestamp ASC
            LIMIT ?
        """, (*chat_aliases, msg_data[0], after))
        
        after_messages = []
        for msg in cursor.fetchall():
            after_messages.append(Message(
                timestamp=datetime.fromisoformat(msg[0]),
                sender=msg[1],
                chat_name=msg[2],
                content=msg[3],
                is_from_me=msg[4],
                chat_jid=msg[5],
                id=msg[6],
                media_type=msg[7],
                sender_name="Me" if msg[4] else get_sender_name(msg[1])
            ))

        return MessageContext(
            message=target_message,
            before=before_messages,
            after=after_messages
        )
        
    except sqlite3.Error as e:
        print(f"Database error: {e}")
        raise
    finally:
        if 'conn' in locals():
            conn.close()


def list_chats(
    query: Optional[str] = None,
    limit: int = 20,
    page: int = 0,
    include_last_message: bool = True,
    sort_by: str = "last_active"
) -> List[Chat]:
    """Get chats, merging phone and LID entries for the same contact."""
    try:
        conn = _connect()
        message_columns = (
            "messages.content, messages.sender, messages.is_from_me"
            if include_last_message else "NULL, NULL, NULL"
        )
        join = (
            "LEFT JOIN messages ON chats.jid = messages.chat_jid "
            "AND chats.last_message_time = messages.timestamp"
            if include_last_message else ""
        )
        rows = conn.execute(f"""
            SELECT chats.jid, chats.name, chats.last_message_time, {message_columns}
            FROM chats {join}
        """).fetchall()

        deduplicated = {}
        for row in rows:
            jid, name = row[0], row[1]
            identity = None if jid.endswith(('@g.us', '@broadcast', '@newsletter')) else _contact_identity(conn, jid)
            canonical_jid = f"{identity[0]}@s.whatsapp.net" if identity else jid
            canonical_name = identity[2] if identity and identity[2] != identity[0] else name
            if query and query.lower() not in f"{canonical_name or ''} {canonical_jid}".lower():
                continue

            chat = _chat_from_row(conn, row)
            previous = deduplicated.get(canonical_jid)
            if not previous or (chat.last_message_time or datetime.min) > (previous.last_message_time or datetime.min):
                deduplicated[canonical_jid] = chat

        result = list(deduplicated.values())
        if sort_by == "last_active":
            result.sort(key=lambda chat: chat.last_message_time or datetime.min, reverse=True)
        else:
            result.sort(key=lambda chat: chat.name or "")
        return result[page * limit:(page + 1) * limit]

    except sqlite3.Error as e:
        print(f"Database error: {e}")
        return []
    finally:
        if 'conn' in locals():
            conn.close()


def search_contacts(query: str) -> List[Contact]:
    """Search canonical contacts by name, phone number, or LID."""
    try:
        conn = _connect()
        pattern = f"%{query}%"
        rows = conn.execute("""
            SELECT phone,
                   COALESCE(NULLIF(display_name, ''), NULLIF(push_name, ''), phone)
            FROM contacts
            WHERE phone LIKE ? OR lid LIKE ?
               OR LOWER(display_name) LIKE LOWER(?) OR LOWER(push_name) LIKE LOWER(?)
            ORDER BY 2, phone
            LIMIT 50
        """, (pattern, pattern, pattern, pattern)).fetchall()
        return [
            Contact(phone_number=phone, name=name, jid=f"{phone}@s.whatsapp.net")
            for phone, name in rows
        ]
    except sqlite3.Error as e:
        print(f"Database error: {e}")
        return []
    finally:
        if 'conn' in locals():
            conn.close()


def get_contact_chats(jid: str, limit: int = 20, page: int = 0) -> List[Chat]:
    """Get all chats involving the contact.
    
    Args:
        jid: The contact's JID to search for
        limit: Maximum number of chats to return (default 20)
        page: Page number for pagination (default 0)
    """
    try:
        conn = _connect()
        aliases = _identity_aliases(conn, jid)
        placeholders = ",".join("?" * len(aliases))
        rows = conn.execute(f"""
            SELECT c.jid, c.name, c.last_message_time,
                   m.content, m.sender, m.is_from_me
            FROM chats c
            LEFT JOIN messages m ON c.jid = m.chat_jid
                AND c.last_message_time = m.timestamp
            WHERE c.jid IN ({placeholders}) OR EXISTS (
                SELECT 1 FROM messages sender_messages
                WHERE sender_messages.chat_jid = c.jid
                  AND sender_messages.sender IN ({placeholders})
            )
            ORDER BY c.last_message_time DESC
            LIMIT ? OFFSET ?
        """, (*aliases, *aliases, limit, page * limit)).fetchall()
        return [_chat_from_row(conn, row) for row in rows]
        
    except sqlite3.Error as e:
        print(f"Database error: {e}")
        return []
    finally:
        if 'conn' in locals():
            conn.close()


def get_last_interaction(jid: str) -> Optional[Message]:
    """Get most recent message involving the contact."""
    try:
        conn = _connect()
        aliases = _identity_aliases(conn, jid)
        placeholders = ",".join("?" * len(aliases))
        msg_data = conn.execute(f"""
            SELECT m.timestamp, m.sender, c.name, m.content,
                   m.is_from_me, c.jid, m.id, m.media_type
            FROM messages m
            JOIN chats c ON m.chat_jid = c.jid
            WHERE m.sender IN ({placeholders}) OR c.jid IN ({placeholders})
            ORDER BY m.timestamp DESC
            LIMIT 1
        """, (*aliases, *aliases)).fetchone()
        
        if not msg_data:
            return None
            
        message = Message(
            timestamp=datetime.fromisoformat(msg_data[0]),
            sender=msg_data[1],
            chat_name=msg_data[2],
            content=msg_data[3],
            is_from_me=msg_data[4],
            chat_jid=msg_data[5],
            id=msg_data[6],
            media_type=msg_data[7],
            sender_name="Me" if msg_data[4] else get_sender_name(msg_data[1])
        )

        return message
        
    except sqlite3.Error as e:
        print(f"Database error: {e}")
        return None
    finally:
        if 'conn' in locals():
            conn.close()


def get_chat(chat_jid: str, include_last_message: bool = True) -> Optional[Chat]:
    """Get chat metadata by canonical JID or one of its aliases."""
    try:
        conn = _connect()
        aliases = _identity_aliases(conn, chat_jid)
        placeholders = ",".join("?" * len(aliases))
        message_columns = "m.content, m.sender, m.is_from_me" if include_last_message else "NULL, NULL, NULL"
        join = (
            "LEFT JOIN messages m ON c.jid = m.chat_jid AND c.last_message_time = m.timestamp"
            if include_last_message else ""
        )
        row = conn.execute(f"""
            SELECT c.jid, c.name, c.last_message_time, {message_columns}
            FROM chats c {join}
            WHERE c.jid IN ({placeholders})
            ORDER BY c.last_message_time DESC
            LIMIT 1
        """, aliases).fetchone()
        return _chat_from_row(conn, row) if row else None
    except sqlite3.Error as e:
        print(f"Database error: {e}")
        return None
    finally:
        if 'conn' in locals():
            conn.close()


def get_direct_chat_by_contact(sender_phone_number: str) -> Optional[Chat]:
    """Get the latest direct chat for a phone number or LID."""
    return get_chat(sender_phone_number)

def send_message(recipient: str, message: str) -> Tuple[bool, str]:
    try:
        # Validate input
        if not recipient:
            return False, "Recipient must be provided"
        
        url = f"{WHATSAPP_API_BASE_URL}/send"
        payload = {
            "recipient": recipient,
            "message": message,
        }

        response = requests.post(url, json=payload, headers=_api_headers())

        success, data = _bridge_response(response)
        return success, data.get("message", "Unknown response")

            
    except requests.RequestException as e:
        return False, f"Request error: {str(e)}"
    except json.JSONDecodeError:
        return False, f"Error parsing response: {response.text}"
    except Exception as e:
        return False, f"Unexpected error: {str(e)}"

def send_file(recipient: str, media_path: str) -> Tuple[bool, str]:
    try:
        # Validate input
        if not recipient:
            return False, "Recipient must be provided"
        
        if not media_path:
            return False, "Media path must be provided"
        
        if not os.path.isfile(media_path):
            return False, f"Media file not found: {media_path}"
        
        url = f"{WHATSAPP_API_BASE_URL}/send"
        payload = {
            "recipient": recipient,
            "media_path": media_path
        }

        response = requests.post(url, json=payload, headers=_api_headers())

        success, data = _bridge_response(response)
        return success, data.get("message", "Unknown response")
            
    except requests.RequestException as e:
        return False, f"Request error: {str(e)}"
    except json.JSONDecodeError:
        return False, f"Error parsing response: {response.text}"
    except Exception as e:
        return False, f"Unexpected error: {str(e)}"

def send_audio_message(recipient: str, media_path: str) -> Tuple[bool, str]:
    try:
        # Validate input
        if not recipient:
            return False, "Recipient must be provided"
        
        if not media_path:
            return False, "Media path must be provided"
        
        if not os.path.isfile(media_path):
            return False, f"Media file not found: {media_path}"

        if not media_path.endswith(".ogg"):
            try:
                media_path = audio.convert_to_opus_ogg_temp(media_path)
            except Exception as e:
                return False, f"Error converting file to opus ogg. You likely need to install ffmpeg: {str(e)}"

        url = f"{WHATSAPP_API_BASE_URL}/send"
        payload = {
            "recipient": recipient,
            "media_path": media_path
        }

        response = requests.post(url, json=payload, headers=_api_headers())

        success, data = _bridge_response(response)
        return success, data.get("message", "Unknown response")
            
    except requests.RequestException as e:
        return False, f"Request error: {str(e)}"
    except json.JSONDecodeError:
        return False, f"Error parsing response: {response.text}"
    except Exception as e:
        return False, f"Unexpected error: {str(e)}"

def download_media(message_id: str, chat_jid: str) -> Optional[str]:
    """Download media from a message and return the local file path.
    
    Args:
        message_id: The ID of the message containing the media
        chat_jid: The JID of the chat containing the message
    
    Returns:
        The local file path if download was successful, None otherwise
    """
    try:
        url = f"{WHATSAPP_API_BASE_URL}/download"
        payload = {
            "message_id": message_id,
            "chat_jid": chat_jid
        }

        response = requests.post(url, json=payload, headers=_api_headers())

        success, data = _bridge_response(response)
        if success:
            path = data.get("path")
            print(f"Media downloaded successfully: {path}")
            return path
        print(f"Download failed: {data.get('message', 'Unknown error')}")
        return None
            
    except requests.RequestException as e:
        print(f"Request error: {str(e)}")
        return None
    except json.JSONDecodeError:
        print(f"Error parsing response: {response.text}")
        return None
    except Exception as e:
        print(f"Unexpected error: {str(e)}")
        return None
