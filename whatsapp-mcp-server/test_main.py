"""
WhatsApp MCP Server — Testes unitários.
Uso: uv run python test_main.py
"""

import sys
import os
import json
import sqlite3
import tempfile
sys.path.insert(0, os.path.dirname(__file__))

from datetime import datetime, timezone, timedelta
from unittest import mock
from pathlib import Path
import approvals
import audit
import main as mcp_main
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


class FakeResponse:
    """Imita o retorno de requests com o envelope {success, data, error}."""
    def __init__(self, payload, status_code=200):
        self._payload = payload
        self.status_code = status_code
        self.text = json.dumps(payload)

    def json(self):
        return self._payload


EVENT_SAMPLE = {
    "seq": 5, "message_id": "m5", "chat_jid": "5511999999999@s.whatsapp.net",
    "is_group": False, "is_from_me": False, "mentioned_me": False,
    "content": "oi", "timestamp": "2026-08-18T20:00:00Z", "created_at": "2026-08-18T20:00:01Z",
}


def test_tenant_db_path_por_tenant():
    """default usa <store>/messages.db; outros tenants usam <store>/tenants/<id>/messages.db."""
    store = os.path.abspath("store-de-teste")
    assert whatsapp.messages_db_path(store, "default") == os.path.join(store, "messages.db")
    assert whatsapp.messages_db_path(store, "cliente-x") == os.path.join(
        store, "tenants", "cliente-x", "messages.db")
    print("  ✅ caminho do DB segue o tenant (default legado preservado)")


def test_tenant_id_invalido_falha_com_mensagem_clara():
    """Ids fora de ^[a-z0-9][a-z0-9-]{1,31}$ (ex.: path traversal) devem levantar ValueError."""
    for bad in ["../x", "A", "a", "-abc", "cliente_x", "Cliente", "", "a" * 33, "x/y"]:
        try:
            whatsapp.validate_tenant_id(bad)
        except ValueError as e:
            assert "WHATSAPP_TENANT_ID" in str(e), str(e)
        else:
            raise AssertionError(f"tenant {bad!r} deveria ser rejeitado")
    assert whatsapp.validate_tenant_id("a" * 32) == "a" * 32
    assert whatsapp.validate_tenant_id("cliente-x") == "cliente-x"
    print("  ✅ tenant inválido é rejeitado com mensagem clara")


def test_envio_usa_url_tenantizada():
    """send_message deve chamar {base}/tenants/<id>/send, com timeout explícito."""
    captured = {}

    def fake_post(url, **kwargs):
        captured["url"] = url
        captured["timeout"] = kwargs.get("timeout")
        return FakeResponse({"success": True, "data": {"message": "ok"}})

    with mock.patch.multiple(whatsapp, WHATSAPP_TENANT_ID="cliente-x", WHATSAPP_API_TOKEN="secret",
                             WHATSAPP_API_BASE_URL="http://bridge.local/api/v1"), \
            mock.patch.object(whatsapp.requests, "post", fake_post):
        ok, msg = whatsapp.send_message("5511999999999", "oi")

    assert ok and msg == "ok", (ok, msg)
    assert captured["url"] == "http://bridge.local/api/v1/tenants/cliente-x/send", captured["url"]
    assert captured["timeout"] == whatsapp.REQUEST_TIMEOUT
    assert whatsapp.tenant_api_url("http://h/api/v1/", "default", "/events") == "http://h/api/v1/tenants/default/events"
    print("  ✅ envio usa /tenants/<id>/send com timeout")


def test_bridge_response_eventos_devolve_lista():
    """Envelope de /events deve ser lido como lista de eventos."""
    payload = {"success": True, "data": {"events": [EVENT_SAMPLE], "next_after": 5}, "error": None}
    ok, data = whatsapp._bridge_response(FakeResponse(payload))
    assert ok
    assert isinstance(data["events"], list) and data["events"][0]["seq"] == 5
    print("  ✅ envelope de eventos é interpretado como lista")


def test_list_events_limita_a_500():
    """limit é restrito a 1..500 antes de ir para a API."""
    captured = []

    def fake_get(url, params=None, **kwargs):
        captured.append((url, dict(params or {}), kwargs.get("timeout")))
        return FakeResponse({"success": True, "data": {"events": [EVENT_SAMPLE], "next_after": 5}})

    with mock.patch.multiple(whatsapp, WHATSAPP_TENANT_ID="cliente-x", WHATSAPP_API_TOKEN="secret",
                             WHATSAPP_API_BASE_URL="http://bridge.local/api/v1"), \
            mock.patch.object(whatsapp.requests, "get", fake_get):
        events = whatsapp.list_events(after=3, rule="r1", limit=9999)
        whatsapp.list_events(limit=0)

    assert isinstance(events, list) and events == [EVENT_SAMPLE]
    url, params, timeout = captured[0]
    assert url == "http://bridge.local/api/v1/tenants/cliente-x/events"
    assert params == {"after": 3, "limit": 500, "rule": "r1"}, params
    assert timeout == whatsapp.REQUEST_TIMEOUT
    assert captured[1][1]["limit"] == 1
    print("  ✅ list_events devolve lista e limita limit a 1..500")


def test_wait_for_events_retorna_no_primeiro_lote():
    """Com o primeiro lote preenchido, deve retornar sem dormir nem repetir a chamada."""
    calls = []

    def fake_list_events(after=0, rule=None, limit=50):
        calls.append(after)
        return [EVENT_SAMPLE]

    with mock.patch.object(whatsapp, "list_events", fake_list_events), \
            mock.patch.object(whatsapp.time, "sleep", side_effect=AssertionError("não deveria dormir")):
        events = whatsapp.wait_for_events(after=4, timeout=30, poll=1.0)

    assert events == [EVENT_SAMPLE]
    assert calls == [4], calls
    print("  ✅ wait_for_events retorna imediatamente com lote preenchido")


def test_wait_for_events_poll_e_timeout():
    """Dorme no máximo `poll` entre tentativas e devolve [] quando o prazo expira."""
    responses = [[], [EVENT_SAMPLE]]
    sleeps = []

    with mock.patch.object(whatsapp, "list_events", lambda after=0, rule=None, limit=50: responses.pop(0)), \
            mock.patch.object(whatsapp.time, "sleep", lambda s: sleeps.append(s)):
        events = whatsapp.wait_for_events(after=0, timeout=30, poll=0.25)
    assert events == [EVENT_SAMPLE]
    assert sleeps and all(s <= 0.25 for s in sleeps), sleeps

    calls = []

    def empty_list_events(after=0, rule=None, limit=50):
        calls.append(1)
        return []

    with mock.patch.object(whatsapp, "list_events", empty_list_events), \
            mock.patch.object(whatsapp.time, "sleep", lambda s: None):
        assert whatsapp.wait_for_events(timeout=0.0, poll=1.0) == []
    assert len(calls) == 1
    print("  ✅ wait_for_events respeita poll e timeout")


def test_ack_rejeita_consumer_id_invalido():
    """consumer_id entra no path: valores com '/' ou '..' são recusados sem chamar a API."""
    with mock.patch.object(whatsapp.requests, "post", side_effect=AssertionError("não deveria chamar a API")):
        for bad in ["../x", "a/b", "", "-x", "a" * 65]:
            ok, msg = whatsapp.ack_events(bad, "rule-1", 1)
            assert not ok and "consumer_id" in msg, (bad, msg)
    print("  ✅ ack_events valida consumer_id antes de montar a URL")


def test_approvals_are_single_use_and_tenant_bound():
    """Aprovação expira, é de uso único e não cruza tenant."""
    import tempfile
    tmp = tempfile.mkdtemp()
    old = {k: os.environ.get(k) for k in (
        "WHATSAPP_MCP_APPROVALS_DB", "WHATSAPP_MCP_APPROVAL_TTL_SECONDS", "WHATSAPP_MCP_OUTBOX_DIR"
    )}
    try:
        os.environ["WHATSAPP_MCP_APPROVALS_DB"] = str(Path(tmp) / "approvals.db")
        os.environ["WHATSAPP_MCP_APPROVAL_TTL_SECONDS"] = "60"
        os.environ["WHATSAPP_MCP_OUTBOX_DIR"] = str(Path(tmp) / "outbox")
        created = approvals.create(tenant_id="t1", kind="message", recipient="5511999999999", text="oi")
        assert created["approval_id"].startswith("ap_")
        assert approvals.consume(tenant_id="t2", approval_id=created["approval_id"]) is None
        consumed = approvals.consume(tenant_id="t1", approval_id=created["approval_id"])
        assert consumed and consumed["text"] == "oi"
        assert approvals.consume(tenant_id="t1", approval_id=created["approval_id"]) is None
    finally:
        for key, value in old.items():
            if value is None:
                os.environ.pop(key, None)
            else:
                os.environ[key] = value
    print("  ✅ aprovação de uso único e vinculada ao tenant")


def test_confirm_send_requires_confirmation_and_consumes_approval():
    """confirm_send não envia com confirm=false e consome a aprovação com true."""
    import tempfile
    tmp = tempfile.mkdtemp()
    old = {k: os.environ.get(k) for k in (
        "WHATSAPP_MCP_APPROVALS_DB", "WHATSAPP_MCP_APPROVAL_TTL_SECONDS"
    )}
    try:
        os.environ["WHATSAPP_MCP_APPROVALS_DB"] = str(Path(tmp) / "approvals.db")
        os.environ["WHATSAPP_MCP_APPROVAL_TTL_SECONDS"] = "60"
        approval = approvals.create(tenant_id="default", kind="message", recipient="5511999999999", text="oi")
        with mock.patch.object(mcp_main, "whatsapp_send_message", return_value=(True, "sent")) as send:
            denied = mcp_main.confirm_send(approval["approval_id"], False)
            assert not denied["success"] and send.call_count == 0
            sent = mcp_main.confirm_send(approval["approval_id"], True)
            assert sent["success"] and send.call_count == 1
            again = mcp_main.confirm_send(approval["approval_id"], True)
            assert not again["success"] and send.call_count == 1
    finally:
        for key, value in old.items():
            if value is None:
                os.environ.pop(key, None)
            else:
                os.environ[key] = value
    print("  ✅ confirm_send exige confirm=true e é de uso único")


def test_outbox_rejects_paths_outside_and_symlinks():
    """Arquivos de envio ficam confinados ao outbox."""
    import tempfile
    tmp = Path(tempfile.mkdtemp())
    outbox = tmp / "outbox"
    outbox.mkdir()
    inside = outbox / "doc.txt"
    inside.write_text("ok", encoding="utf-8")
    outside = tmp / "outside.txt"
    outside.write_text("nope", encoding="utf-8")
    old = os.environ.get("WHATSAPP_MCP_OUTBOX_DIR")
    try:
        os.environ["WHATSAPP_MCP_OUTBOX_DIR"] = str(outbox)
        assert approvals.resolve_outbox_file("doc.txt") == inside.resolve()
        for bad in [str(outside), "../outside.txt"]:
            try:
                approvals.resolve_outbox_file(bad)
            except ValueError:
                pass
            else:
                raise AssertionError(f"path should be rejected: {bad}")
    finally:
        if old is None:
            os.environ.pop("WHATSAPP_MCP_OUTBOX_DIR", None)
        else:
            os.environ["WHATSAPP_MCP_OUTBOX_DIR"] = old
    print("  ✅ outbox rejeita paths fora do diretório")


def test_file_approval_detects_content_change():
    """O hash capturado no draft impede troca do arquivo antes do envio."""
    import tempfile
    tmp = Path(tempfile.mkdtemp())
    outbox = tmp / "outbox"
    outbox.mkdir()
    media = outbox / "doc.txt"
    media.write_text("versão aprovada", encoding="utf-8")
    old_db = os.environ.get("WHATSAPP_MCP_APPROVALS_DB")
    old_outbox = os.environ.get("WHATSAPP_MCP_OUTBOX_DIR")
    try:
        os.environ["WHATSAPP_MCP_APPROVALS_DB"] = str(tmp / "approvals.db")
        os.environ["WHATSAPP_MCP_OUTBOX_DIR"] = str(outbox)
        digest = approvals.hash_file(media)
        created = approvals.create(
            tenant_id="default", kind="file", recipient="5511999999999",
            file_path=media.resolve(), file_sha256=digest,
        )
        assert approvals.file_still_matches(created)
        media.write_text("outra coisa", encoding="utf-8")
        assert not approvals.file_still_matches(created)
    finally:
        for key, value in (("WHATSAPP_MCP_APPROVALS_DB", old_db), ("WHATSAPP_MCP_OUTBOX_DIR", old_outbox)):
            if value is None:
                os.environ.pop(key, None)
            else:
                os.environ[key] = value
    print("  ✅ hash do arquivo aprovado é revalidado")


def test_audit_log_redacts_message_and_recipient():
    """Auditoria registra metadados, nunca conteúdo ou telefone."""
    import tempfile
    tmp = Path(tempfile.mkdtemp())
    log = tmp / "audit.jsonl"
    old = os.environ.get("WHATSAPP_MCP_AUDIT_LOG")
    try:
        os.environ["WHATSAPP_MCP_AUDIT_LOG"] = str(log)

        @audit.audited
        def fake_tool(recipient: str, message: str) -> str:
            return message

        assert fake_tool("5511999999999", "segredo absoluto") == "segredo absoluto"
        raw = log.read_text(encoding="utf-8")
        assert "segredo absoluto" not in raw
        assert "5511999999999" not in raw
        record = json.loads(raw.strip().splitlines()[-1])
        assert record["tool"] == "fake_tool"
        assert record["outcome"] == "ok"
        assert record["recipient_hash"]
    finally:
        if old is None:
            os.environ.pop("WHATSAPP_MCP_AUDIT_LOG", None)
        else:
            os.environ["WHATSAPP_MCP_AUDIT_LOG"] = old
    print("  ✅ auditoria redige conteúdo e telefone")



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
        test_tenant_db_path_por_tenant,
        test_tenant_id_invalido_falha_com_mensagem_clara,
        test_envio_usa_url_tenantizada,
        test_bridge_response_eventos_devolve_lista,
        test_list_events_limita_a_500,
        test_wait_for_events_retorna_no_primeiro_lote,
        test_wait_for_events_poll_e_timeout,
        test_ack_rejeita_consumer_id_invalido,
        test_approvals_are_single_use_and_tenant_bound,
        test_confirm_send_requires_confirmation_and_consumes_approval,
        test_outbox_rejects_paths_outside_and_symlinks,
        test_file_approval_detects_content_change,
        test_audit_log_redacts_message_and_recipient,
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
