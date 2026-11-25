package main

import (
	"testing"
	"time"

	waProto "go.mau.fi/whatsmeow/binary/proto"
)

func strPtr(value string) *string { return &value }

func TestExtractTextContent(t *testing.T) {
	tests := []struct {
		name string
		msg  *waProto.Message
		want string
	}{
		{name: "nil", msg: nil, want: ""},
		{name: "conversation", msg: &waProto.Message{Conversation: strPtr("oi")}, want: "oi"},
		{name: "extended text", msg: &waProto.Message{
			ExtendedTextMessage: &waProto.ExtendedTextMessage{Text: strPtr("texto estendido")},
		}, want: "texto estendido"},
		{name: "image caption", msg: &waProto.Message{
			ImageMessage: &waProto.ImageMessage{Caption: strPtr("legenda")},
		}, want: "legenda"},
		{name: "sticker", msg: &waProto.Message{StickerMessage: &waProto.StickerMessage{}}, want: "[sticker]"},
		{name: "empty", msg: &waProto.Message{}, want: ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := extractTextContent(tc.msg); got != tc.want {
				t.Fatalf("extractTextContent() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestMessageTypeOf(t *testing.T) {
	tests := []struct {
		name      string
		msg       *waProto.Message
		mediaType string
		content   string
		want      string
	}{
		{name: "media type wins", msg: nil, mediaType: "image", want: "image"},
		{name: "nil with text", msg: nil, content: "hello", want: "text"},
		{name: "nil empty", msg: nil, want: "other"},
		{name: "sticker", msg: &waProto.Message{StickerMessage: &waProto.StickerMessage{}}, want: "sticker"},
		{name: "location", msg: &waProto.Message{
			LocationMessage: &waProto.LocationMessage{},
		}, want: "location"},
		{name: "contact", msg: &waProto.Message{
			ContactMessage: &waProto.ContactMessage{},
		}, want: "contact"},
		{name: "text body", msg: &waProto.Message{Conversation: strPtr("oi")}, content: "oi", want: "text"},
		{name: "empty body", msg: &waProto.Message{}, want: "other"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := messageTypeOf(tc.msg, tc.mediaType, tc.content); got != tc.want {
				t.Fatalf("messageTypeOf() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestWebhookBackoff(t *testing.T) {
	tests := []struct {
		attempts int
		want     time.Duration
	}{
		{attempts: 0, want: 2 * time.Second},
		{attempts: 1, want: 2 * time.Second},
		{attempts: 2, want: 4 * time.Second},
		{attempts: 3, want: 8 * time.Second},
		{attempts: 8, want: 256 * time.Second},
		{attempts: 9, want: 5 * time.Minute},
		{attempts: 20, want: 5 * time.Minute},
		{attempts: 21, want: 5 * time.Minute},
	}
	for _, tc := range tests {
		if got := webhookBackoff(tc.attempts); got != tc.want {
			t.Errorf("webhookBackoff(%d) = %s, want %s", tc.attempts, got, tc.want)
		}
	}
}

func TestTenantKeyViews(t *testing.T) {
	now := time.Now().UTC()
	lastUsed := now.Add(-time.Minute)
	revoked := now.Add(-time.Hour)
	tenant := Tenant{
		ID: "tenant-a",
		Keys: []TenantKey{
			{ID: "key-1", Label: "one", Hash: "must-not-leak", CreatedAt: now, LastUsedAt: &lastUsed},
			{ID: "key-2", Label: "two", Hash: "must-not-leak", CreatedAt: now, RevokedAt: &revoked},
		},
	}

	views := tenantKeyViews(tenant)
	if len(views) != 2 {
		t.Fatalf("len(views) = %d, want 2", len(views))
	}
	if views[0].ID != "key-1" || views[0].LastUsedAt == nil || views[0].RevokedAt != nil {
		t.Fatalf("unexpected first key view: %+v", views[0])
	}
	if views[1].ID != "key-2" || views[1].RevokedAt == nil || views[1].LastUsedAt != nil {
		t.Fatalf("unexpected second key view: %+v", views[1])
	}
}
