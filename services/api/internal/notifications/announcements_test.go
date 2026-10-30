package notifications

import "testing"

func TestNotificationContentReferencesAndMailHeaderBoundaries(t *testing.T) {
	for _, v := range []struct {
		title, body string
		ids         []int64
	}{
		{"Title\r\nBcc: injected", "Body", nil}, {"  ", "Body", nil}, {"Title", " ", nil}, {"Title", "[x](file:9)", []int64{8}}, {"Title", "Body", []int64{2, 2}}, {"Title", "Body", []int64{0}},
	} {
		if validateContent(v.title, v.body, v.ids) == nil {
			t.Fatal("unsafe notification accepted", v.title, v.body)
		}
	}
	if err := validateContent("Normal title", "# Markdown\n[File](file:9)", []int64{9}); err != nil {
		t.Fatal(err)
	}
}
