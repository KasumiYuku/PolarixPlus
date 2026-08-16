package main

import (
	"Plrx/lib/structers"
	"encoding/json"
	"testing"
)

func TestMessageAttachmentsDecode(t *testing.T) {
	var payload structers.Payload
	err := json.Unmarshal([]byte(`{
		"d": {
			"content": "/draw 改成水彩风格",
			"attachments": [{
				"content_type": "image/png",
				"filename": "reference.png",
				"height": 768,
				"width": 1024,
				"size": 12345,
				"url": "https://example.com/reference.png"
			}]
		}
	}`), &payload)
	if err != nil {
		t.Fatal(err)
	}
	if len(payload.Data.Attachments) != 1 {
		t.Fatalf("expected one attachment, got %d", len(payload.Data.Attachments))
	}
	attachment := payload.Data.Attachments[0]
	if attachment.ContentType != "image/png" || attachment.URL != "https://example.com/reference.png" {
		t.Fatalf("unexpected attachment: %#v", attachment)
	}
}
