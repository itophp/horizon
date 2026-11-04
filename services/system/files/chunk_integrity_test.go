package files

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"testing"
)

func TestVerifyChunk(t *testing.T) {
	content := []byte("chunk")
	sum := sha256.Sum256(content)
	for _, tc := range []struct {
		name string
		data []byte
		size int64
		sum  [sha256.Size]byte
		ok   bool
	}{
		{"valid", content, int64(len(content)), sum, true},
		{"empty", nil, 0, sha256.Sum256(nil), true},
		{"short", content, 6, sum, false},
		{"long", content, 4, sum, false},
		{"digest", content, 5, sha256.Sum256(nil), false},
		{"negative", content, -1, sum, false},
		{"oversized", content, CloudDriveChunkLimit + 1, sum, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			err := VerifyChunk(&out, bytes.NewReader(tc.data), tc.size, tc.sum)
			if tc.ok && (err != nil || !bytes.Equal(out.Bytes(), tc.data)) {
				t.Fatalf("valid chunk: %v", err)
			}
			if !tc.ok && !errors.Is(err, ErrChunkIntegrity) {
				t.Fatalf("expected integrity error, got %v", err)
			}
		})
	}
}

func TestFileDigest(t *testing.T) {
	d := NewFileDigest()
	_, _ = d.Write([]byte("ab"))
	_, _ = d.Write([]byte("cd"))
	if d.Size() != 4 || d.Sum() != sha256.Sum256([]byte("abcd")) {
		t.Fatal("incorrect incremental digest")
	}
}
