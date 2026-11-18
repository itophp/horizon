package knowledge

import (
	"strings"
	"testing"
)

func TestSplitProducesOverlappingWindows(t *testing.T) {
	text := strings.Repeat("知", 1300)
	chunks := Split(text, 600, 80)
	if len(chunks) != 3 {
		t.Fatalf("expected 3 chunks, got %d", len(chunks))
	}
	for _, chunk := range chunks {
		if runes := len([]rune(chunk)); runes > 600 {
			t.Fatalf("chunk exceeds the window: %d", runes)
		}
	}
	if chunks[0] == "" || chunks[len(chunks)-1] == "" {
		t.Fatal("empty chunk produced")
	}
	if Split("   ", 600, 80) != nil {
		t.Fatal("blank input must produce no chunks")
	}
}

func TestSplitCapsTheChunkCount(t *testing.T) {
	chunks := Split(strings.Repeat("a", 4000*maxChunks), chunkRunes, chunkOverlap)
	if len(chunks) != maxChunks {
		t.Fatalf("expected the cap %d, got %d", maxChunks, len(chunks))
	}
}

func TestWorkspaceTagFilterTreatsWorkspaceIDAsOneLiteral(t *testing.T) {
	for _, tt := range []struct{ tenant, want string }{
		{"workspace", `@tenant:{workspace}`},
		{"team-west", `@tenant:{team\-west}`},
		{`研发 team}|{other\*`, `@tenant:{研发\ team\}\|\{other\\\*}`},
	} {
		if got := workspaceTagFilter(tt.tenant); got != tt.want {
			t.Errorf("workspaceTagFilter(%q) = %q, want %q", tt.tenant, got, tt.want)
		}
	}
}
