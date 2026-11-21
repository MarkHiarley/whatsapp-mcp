package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestSanitizeChatDir(t *testing.T) {
	for name, test := range map[string]struct {
		input  string
		want   string
		absent []string
	}{
		"phone":          {"5511999999999@s.whatsapp.net", "5511999999999@s.whatsapp.net", nil},
		"device":         {"5511999999999:12@s.whatsapp.net", "5511999999999_12@s.whatsapp.net", nil},
		"path traversal": {"../../etc/passwd", "__etc_passwd", []string{"..", "/"}},
		"separators":     {`a/b\c`, "a_b_c", []string{"/", `\`}},
		"lone dots":      {"..", "_", []string{".."}},
	} {
		t.Run(name, func(t *testing.T) {
			got := sanitizeChatDir(test.input)
			if got != test.want {
				t.Fatalf("sanitizeChatDir(%q) = %q, want %q", test.input, got, test.want)
			}
			for _, absent := range test.absent {
				if strings.Contains(got, absent) {
					t.Fatalf("sanitizeChatDir(%q) = %q still contains %q", test.input, got, absent)
				}
			}
		})
	}
}

func TestMediaPathStaysUnderMediaDir(t *testing.T) {
	base := t.TempDir()
	joined := filepath.Join(base, sanitizeChatDir("../../escape"))
	absBase, err := filepath.Abs(base)
	if err != nil {
		t.Fatal(err)
	}
	absJoined, err := filepath.Abs(joined)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(absJoined, absBase+string(filepath.Separator)) {
		t.Fatalf("sanitized path escaped the media directory: %s", absJoined)
	}
}

func TestExtractDirectPathKeepsQueryString(t *testing.T) {
	const raw = "https://mmg.whatsapp.net/o1/v/t24/f2/m235/PAYLOAD?ccb=11-4&oh=abc&oe=xyz"
	want := "/o1/v/t24/f2/m235/PAYLOAD?ccb=11-4&oh=abc&oe=xyz"
	if got := extractDirectPathFromURL(raw); got != want {
		t.Fatalf("extractDirectPathFromURL() = %q, want %q", got, want)
	}
}
