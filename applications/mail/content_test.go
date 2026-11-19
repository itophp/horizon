package mail

import (
	"strings"
	"testing"
)

func TestRenderMarkdownSanitizesHTML(t *testing.T) {
	text, html, err := RenderMarkdown("# Title\n\n<script>alert(1)</script>\n\n[unsafe](javascript:alert(1)) [safe](https://example.test)")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "# Title") {
		t.Fatal("plain markdown was changed")
	}
	if strings.Contains(html, "<script>") || strings.Contains(html, "javascript:") {
		t.Fatalf("unsafe HTML survived: %s", html)
	}
	if !strings.Contains(html, "<h1>Title</h1>") || !strings.Contains(html, `href="https://example.test"`) {
		t.Fatalf("safe markdown was not rendered: %s", html)
	}
}

func TestSanitizeHTMLRejectsOversizeInput(t *testing.T) {
	if _, err := SanitizeHTML("<p>"+strings.Repeat("a", 11)+"</p>", 10); err == nil {
		t.Fatal("oversize HTML was accepted")
	}
}
