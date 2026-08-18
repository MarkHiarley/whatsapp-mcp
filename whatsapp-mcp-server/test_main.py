"""
WhatsApp MCP Server — Testes unitários.
Uso: uv run python test_main.py
"""

import sys
import os
import sqlite3
import tempfile
sys.path.insert(0, os.path.dirname(__file__))

from datetime import datetime, timezone, timedelta
import whatsapp
from main import _contract
from whatsapp import Message
from whatsapp import _bridge_response, _identity_aliases, _localize_dt, _tz_name, format_message


# ─── Testes ──────────────────────────────────────────────────
def test_localize_dt_naive():
    """datetime naive deve virar timezone-aware (assume UTC, converte pra local)."""
    naive = datetime(2026, 7, 23, 10, 0, 0)
    aware = _localize_dt(naive)
    assert aware.tzinfo is not None, "Deveria ter timezone após localizar"
    assert aware.hour != naive.hour, "Deveria converter pra fuso local"
    print(f"  ✅ naive {naive} → {aware}")


def test_localize_dt_none():
    """None deve retornar None."""
    assert _localize_dt(None) is None
    print("  ✅ None retorna None")


def test_localize_dt_aware():
    """datetime já aware deve manter timezone mas converter pra local."""
    utc_dt = datetime(2026, 7, 23, 10, 0, 0, tzinfo=timezone.utc)
    local_dt = _localize_dt(utc_dt)
    assert local_dt.tzinfo is not None
    # Horário deve ser diferente do UTC se o fuso não for UTC
    offset = local_dt.utcoffset()
    if offset and offset != timedelta(0):
        assert local_dt.hour != utc_dt.hour, "Deveria converter pra fuso local"
    print(f"  ✅ aware {utc_dt} → {local_dt}")


def test_tz_name_not_empty():
    """_tz_name deve ter um valor."""
    assert _tz_name and len(_tz_name) > 0, "Nome do timezone não pode ser vazio"
    print(f"  ✅ _tz_name = {_tz_name}")


def test_format_message_tz():
    """format_message deve incluir timezone."""
    msg = Message(
        id="test123",
        timestamp=datetime(2026, 7, 23, 10, 0, 0),
        sender="5511999999999@s.whatsapp.net",
        content="Mensagem de teste",
        is_from_me=True,
        chat_jid="",
        chat_name="Teste",
    )
    output = format_message(msg, show_chat_info=True)
    assert _tz_name in output, f"Timezone {_tz_name} deveria aparecer na saída"
    assert "Teste" in output, "Chat name deveria aparecer"
    assert "Mensagem de teste" in output, "Conteúdo deveria aparecer"
    print(f"  ✅ {output.strip()}")


def test_format_message_no_chat():
    """format_message sem chat_name."""
    msg = Message(
        timestamp=datetime(2026, 7, 23, 10, 0, 0),
        sender="5511999999999@s.whatsapp.net",
        content="Teste",
        is_from_me=True,
        chat_jid="",
        id="",
    )
    output = format_message(msg, show_chat_info=False)
    assert _tz_name in output
    print(f"  ✅ {output.strip()}")


def test_format_message_media():
    """format_message com midia."""
    msg = Message(
        timestamp=datetime(2026, 7, 23, 10, 0, 0),
        sender="5511999999999@s.whatsapp.net",
        content="",
        is_from_me=False,
        media_type="IMAGE",
        id="media123",
        chat_jid="5511999999999@s.whatsapp.net",
    )
    output = format_message(msg, show_chat_info=False)
    assert _tz_name in output
    assert "IMAGE" in output
    print(f"  ✅ {output.strip()}")


def test_mcp_message_contract():
    """Mensagem MCP deve ser JSON estruturado com timestamp ISO-8601."""
    payload = _contract(Message(
        id="contract-1",
        timestamp=datetime(2026, 8, 18, 20, 0, 0),
        sender="5511999999999",
        sender_name="Contato",
        content="Olá",
        is_from_me=False,
        chat_jid="5511999999999@s.whatsapp.net",
        chat_name="Contato",
    ))
    assert set(payload) == {
        "id", "timestamp", "sender", "sender_name", "content", "is_from_me",
        "chat_jid", "chat_name", "media_type"
    }
    assert datetime.fromisoformat(payload["timestamp"]).tzinfo is not None
    print("  ✅ contrato MCP retorna mensagem estruturada")


def test_bridge_response_contract():
    """Cliente Python deve consumir o envelope da API v1."""
    class Response:
        status_code = 400

        @staticmethod
        def json():
            return {
                "success": False,
                "data": None,
                "error": {"code": "INVALID_REQUEST", "message": "Recipient is required"},
            }

    success, data = _bridge_response(Response())
    assert not success
    assert data == {"message": "Recipient is required"}
    print("  ✅ cliente Python interpreta envelope da API v1")


def test_api_token_header():
    """Chamadas REST devem enviar o token configurado."""
    original_token = whatsapp.WHATSAPP_API_TOKEN
    whatsapp.WHATSAPP_API_TOKEN = "secret"
    try:
        assert whatsapp._api_headers() == {"Authorization": "Bearer secret"}
    finally:
        whatsapp.WHATSAPP_API_TOKEN = original_token
    print("  ✅ token REST enviado como Bearer")


def test_lid_identity_aliases():
    """Telefone e LID devem resolver para a mesma identidade e chat."""
    with tempfile.NamedTemporaryFile(suffix=".db") as db:
        conn = sqlite3.connect(db.name)
        conn.executescript("""
            CREATE TABLE contacts (phone TEXT, lid TEXT, display_name TEXT, push_name TEXT);
            CREATE TABLE chats (jid TEXT PRIMARY KEY, name TEXT, last_message_time TEXT);
            CREATE TABLE messages (
                id TEXT, chat_jid TEXT, sender TEXT, content TEXT, timestamp TEXT,
                is_from_me BOOLEAN, media_type TEXT
            );
            INSERT INTO contacts VALUES ('5511999999999', '123456789', 'Contato', '');
            INSERT INTO chats VALUES ('123456789@lid', '123456789', '2026-01-01 10:00:00');
            INSERT INTO chats VALUES ('5511999999999@s.whatsapp.net', 'Contato', '2026-01-02 10:00:00');
        """)
        conn.commit()
        expected = {'5511999999999', '123456789', '5511999999999@s.whatsapp.net', '123456789@lid'}
        assert set(_identity_aliases(conn, '123456789@lid')) == expected
        conn.close()

        original_path = whatsapp.MESSAGES_DB_PATH
        whatsapp.MESSAGES_DB_PATH = db.name
        try:
            chats = whatsapp.list_chats()
            assert len(chats) == 1
            assert chats[0].jid == '5511999999999@s.whatsapp.net'
            assert chats[0].name == 'Contato'
        finally:
            whatsapp.MESSAGES_DB_PATH = original_path
    print("  ✅ telefone e LID compartilham a mesma identidade e chat")


# ─── Rodar ───────────────────────────────────────────────────
if __name__ == "__main__":
    print(f"\n🔍 Testes do WhatsApp MCP (fuso: {_tz_name})\n")
    tests = [
        test_localize_dt_naive,
        test_localize_dt_none,
        test_localize_dt_aware,
        test_tz_name_not_empty,
        test_format_message_tz,
        test_format_message_no_chat,
        test_format_message_media,
        test_mcp_message_contract,
        test_bridge_response_contract,
        test_api_token_header,
        test_lid_identity_aliases,
    ]
    passed = 0
    for t in tests:
        try:
            t()
            passed += 1
        except Exception as e:
            print(f"  ❌ {t.__name__}: {e}")
    print(f"\n{'='*40}\nResultado: {passed}/{len(tests)} passaram\n")
    if passed != len(tests):
        raise SystemExit(1)
