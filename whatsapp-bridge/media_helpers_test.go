package main

import (
	"bytes"
	"strings"
	"testing"

	waProto "go.mau.fi/whatsmeow/binary/proto"
)

func TestExtractMediaInfo(t *testing.T) {
	key := []byte{1, 2, 3}
	sha := []byte{4, 5, 6}
	enc := []byte{7, 8, 9}
	length := uint64(123)

	t.Run("image", func(t *testing.T) {
		msg := &waProto.Message{ImageMessage: &waProto.ImageMessage{
			URL:           strPtr("https://mmg.example/image"),
			MediaKey:      key,
			FileSHA256:    sha,
			FileEncSHA256: enc,
			FileLength:    &length,
		}}
		mediaType, filename, url, gotKey, gotSHA, gotEnc, gotLength := extractMediaInfo(msg)
		if mediaType != "image" || !strings.HasPrefix(filename, "image_") || !strings.HasSuffix(filename, ".jpg") {
			t.Fatalf("unexpected image metadata: type=%q filename=%q", mediaType, filename)
		}
		if url != "https://mmg.example/image" || !bytes.Equal(gotKey, key) ||
			!bytes.Equal(gotSHA, sha) || !bytes.Equal(gotEnc, enc) || gotLength != length {
			t.Fatalf("unexpected image payload: url=%q key=%v sha=%v enc=%v length=%d", url, gotKey, gotSHA, gotEnc, gotLength)
		}
	})

	t.Run("document keeps original filename", func(t *testing.T) {
		msg := &waProto.Message{DocumentMessage: &waProto.DocumentMessage{
			FileName: strPtr("report.pdf"),
			URL:      strPtr("https://mmg.example/doc"),
		}}
		mediaType, filename, url, _, _, _, _ := extractMediaInfo(msg)
		if mediaType != "document" || filename != "report.pdf" || url != "https://mmg.example/doc" {
			t.Fatalf("unexpected document metadata: type=%q filename=%q url=%q", mediaType, filename, url)
		}
	})

	t.Run("unsupported", func(t *testing.T) {
		mediaType, filename, url, key, sha, enc, length := extractMediaInfo(&waProto.Message{
			Conversation: strPtr("texto"),
		})
		if mediaType != "" || filename != "" || url != "" || key != nil || sha != nil || enc != nil || length != 0 {
			t.Fatalf("unsupported message returned media metadata")
		}
	})

	t.Run("nil", func(t *testing.T) {
		if mediaType, _, _, _, _, _, _ := extractMediaInfo(nil); mediaType != "" {
			t.Fatalf("nil message returned media type %q", mediaType)
		}
	})
}

func TestMediaDownloaderAccessors(t *testing.T) {
	downloader := &MediaDownloader{
		URL:           "https://mmg.example/media",
		DirectPath:    "/direct/path",
		MediaKey:      []byte{1},
		FileLength:    42,
		FileSHA256:    []byte{2},
		FileEncSHA256: []byte{3},
	}
	if downloader.GetURL() != downloader.URL ||
		downloader.GetDirectPath() != downloader.DirectPath ||
		!bytes.Equal(downloader.GetMediaKey(), downloader.MediaKey) ||
		downloader.GetFileLength() != downloader.FileLength ||
		!bytes.Equal(downloader.GetFileSHA256(), downloader.FileSHA256) ||
		!bytes.Equal(downloader.GetFileEncSHA256(), downloader.FileEncSHA256) {
		t.Fatal("media downloader accessor returned a different value")
	}
}
