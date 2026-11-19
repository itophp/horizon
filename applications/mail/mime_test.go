package mail

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/emersion/go-message"
)

func TestParseMIMEAlternativesAndAttachment(t *testing.T) {
	raw := strings.Join([]string{
		"From: Ada <ada@example.test>",
		"To: Grace <grace@example.test>",
		"Cc: Alan <alan@example.test>",
		"Subject: Fixture",
		"Date: Mon, 02 Jan 2006 15:04:05 -0700",
		"Message-Id: <fixture@example.test>",
		"MIME-Version: 1.0",
		`Content-Type: multipart/mixed; boundary="mixed"`,
		"",
		"--mixed",
		`Content-Type: multipart/alternative; boundary="alt"`,
		"",
		"--alt",
		"Content-Type: text/plain; charset=utf-8",
		"",
		"Plain body",
		"--alt",
		"Content-Type: text/html; charset=utf-8",
		"",
		"<p onclick=\"alert(1)\">HTML <script>alert(2)</script>body</p><img src=\"https://tracker.test/x.png\" alt=\"tracker\">",
		"--alt--",
		"--mixed",
		"Content-Type: text/plain; name=\"notes.txt\"",
		"Content-Disposition: attachment; filename=\"notes.txt\"",
		"",
		"attachment body",
		"--mixed--",
		"",
	}, "\r\n")
	entity, err := message.Read(bytes.NewReader([]byte(raw)))
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseMIME(entity, 1000000)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.From != "ada@example.test" || len(parsed.To) != 1 || len(parsed.CC) != 1 {
		t.Fatalf("addresses were not parsed: %+v", parsed)
	}
	if parsed.Text != "Plain body" {
		t.Fatalf("plain body = %q", parsed.Text)
	}
	if strings.Contains(parsed.HTML, "onclick") || strings.Contains(parsed.HTML, "<script") || strings.Contains(parsed.HTML, "<img") {
		t.Fatalf("unsafe HTML survived: %s", parsed.HTML)
	}
	if !strings.Contains(parsed.HTML, "HTML body") {
		t.Fatalf("safe HTML content was lost: %s", parsed.HTML)
	}
	if len(parsed.Attachments) != 1 || parsed.Attachments[0].Filename != "notes.txt" || string(parsed.Attachments[0].Data) != "attachment body" {
		t.Fatalf("attachment was not parsed: %+v", parsed.Attachments)
	}
}

func TestParseMIMERejectsTotalMessageLimit(t *testing.T) {
	entity, err := message.Read(bytes.NewReader([]byte("Content-Type: text/plain; charset=utf-8\r\n\r\n" + strings.Repeat("x", 11))))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseMIME(entity, 10); !errors.Is(err, ErrMimeTooLarge) {
		t.Fatalf("err=%v", err)
	}
}
