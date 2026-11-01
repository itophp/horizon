package articles

import (
	"baseline/services/api/internal/platform/capability/registry"
	"testing"
)

func TestBlockedWords(t *testing.T) {
	for _, test := range []struct {
		text, words string
		want        bool
	}{{"Hello", "bad", false}, {"B.a D", "bad", true}, {"敏\u200b感 词", "敏感词", true}, {"hello", "\n ,", false}, {"a fraud", "foo,frAUD", true}} {
		if got := ContainsBlocked(test.text, test.words); got != test.want {
			t.Fatalf("match=%v want %v", got, test.want)
		}
	}
}
func TestCatalog(t *testing.T) {
	r := registry.New((&Module{}).Entries())
	if err := r.Err(); err != nil {
		t.Fatal(err)
	}
}
