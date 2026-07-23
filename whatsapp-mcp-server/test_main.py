"""
WhatsApp MCP Server — Testes unitários.
Uso: uv run python test_main.py
"""

import sys
import os
sys.path.insert(0, os.path.dirname(__file__))

from datetime import datetime, timezone, timedelta
from whatsapp import _localize_dt, _tz_name, format_message
from dataclasses import dataclass
from typing import Optional, List


# ─── Mock das classes do whatsapp.py ─────────────────────────
@dataclass
class Message:
    id: str = ""
    timestamp: datetime = datetime.now()
    sender: str = ""
    content: str = ""
    is_from_me: bool = False
    chat_jid: str = ""
    chat_name: str = ""
    media_type: Optional[str] = None
    filename: Optional[str] = None


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
    ]
    passed = 0
    for t in tests:
        try:
            t()
            passed += 1
        except Exception as e:
            print(f"  ❌ {t.__name__}: {e}")
    print(f"\n{'='*40}\nResultado: {passed}/{len(tests)} passaram\n")
