package mail

import (
	"bytes"
	"fmt"
	stdhtml "html"
	"regexp"
	"strings"
	"unicode/utf8"

	"horizon/foundation/system/governance"

	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	goldhtml "github.com/yuin/goldmark/renderer/html"
)

var whitespace = regexp.MustCompile(`[ \t\r\n]+`)

func mailHTMLPolicy() *bluemonday.Policy {
	p := bluemonday.NewPolicy()
	p.AllowElements("a", "b", "blockquote", "br", "code", "dd", "div", "dl", "dt", "em",
		"h1", "h2", "h3", "h4", "h5", "h6", "hr", "i", "li", "ol", "p", "pre", "s",
		"strong", "sub", "sup", "table", "tbody", "td", "th", "thead", "tr", "ul")
	p.AllowAttrs("href").OnElements("a")
	p.AllowAttrs("colspan", "rowspan").Matching(bluemonday.Integer).OnElements("td", "th")
	p.AllowURLSchemes("http", "https", "mailto")
	p.RequireNoFollowOnLinks(true)
	return p
}

func SanitizeHTML(value string, limit int) (string, error) {
	if len(value) > limit {
		return "", governance.Problem("invalid")
	}
	out := mailHTMLPolicy().Sanitize(strings.ToValidUTF8(value, ""))
	if len(out) > limit {
		return "", governance.Problem("invalid")
	}
	return out, nil
}

func RenderMarkdown(markdown string) (string, string, error) {
	if strings.TrimSpace(markdown) == "" || !utf8.ValidString(markdown) {
		return "", "", governance.Problem("invalid")
	}
	var rendered bytes.Buffer
	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
		goldmark.WithRendererOptions(goldhtml.WithHardWraps()),
	)
	if err := md.Convert([]byte(markdown), &rendered); err != nil {
		return "", "", err
	}
	safe, err := SanitizeHTML(rendered.String(), 1000000)
	if err != nil {
		return "", "", err
	}
	return strings.ToValidUTF8(markdown, ""), safe, nil
}

func HTMLToText(value string) string {
	text := mailHTMLPolicy().Sanitize(value)
	text = strings.ReplaceAll(text, "</p>", "\n")
	text = strings.ReplaceAll(text, "<br>", "\n")
	text = strings.ReplaceAll(text, "<br/>", "\n")
	text = regexp.MustCompile(`<[^>]*>`).ReplaceAllString(text, "")
	text = stdhtml.UnescapeString(text)
	return strings.ToValidUTF8(text, "")
}

func Snippet(text string) string {
	value := strings.TrimSpace(whitespace.ReplaceAllString(text, " "))
	runes := []rune(value)
	if len(runes) > 200 {
		return string(runes[:200])
	}
	return value
}

func endpoint(host string, port int) string {
	return fmt.Sprintf("%s:%d", host, port)
}
