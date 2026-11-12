package files

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalStorageContainsPathsAndCleansFailedWrites(t *testing.T) {
	root := t.TempDir()
	driver := Local{Root: root}
	ctx := context.Background()
	profile := Profile{Driver: "local", Directory: "tenant/uploads"}
	if err := driver.Write(ctx, profile, "safe", strings.NewReader("content")); err != nil {
		t.Fatal(err)
	}
	reader, err := driver.Read(ctx, profile, "safe")
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(reader)
	reader.Close()
	if err != nil || string(data) != "content" {
		t.Fatal("read", err)
	}
	for _, key := range []string{"../outside", "/absolute", "a/b", "a\\b"} {
		if err = driver.Write(ctx, profile, key, strings.NewReader("bad")); err == nil {
			t.Fatal("path accepted", key)
		}
	}
	outside := t.TempDir()
	if err = os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Fatal(err)
	}
	if err = driver.Write(ctx, Profile{Driver: "local", Directory: "escape"}, "bad", strings.NewReader("bad")); err == nil {
		t.Fatal("symlink escaped root")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if err = driver.Write(cancelled, profile, "cancelled", strings.NewReader("partial")); err == nil {
		t.Fatal("cancel ignored")
	}
	if _, err = os.Stat(filepath.Join(root, profile.Directory, "cancelled.tmp")); !os.IsNotExist(err) {
		t.Fatal("partial upload remained", err)
	}
}

func TestLocalRangeAndInterruptedTemporaryWrite(t *testing.T) {
	root := t.TempDir()
	driver := Local{Root: root}
	profile := Profile{Driver: "local", Directory: "tenant/uploads"}
	ctx := context.Background()
	if err := driver.Write(ctx, profile, "file", strings.NewReader("0123456789")); err != nil {
		t.Fatal(err)
	}
	reader, err := driver.ReadRange(ctx, profile, "file", 3, 4)
	if err != nil {
		t.Fatal(err)
	}
	data, readErr := io.ReadAll(reader)
	closeErr := reader.Close()
	if readErr != nil || closeErr != nil || string(data) != "3456" {
		t.Fatalf("range = %q, read=%v close=%v", data, readErr, closeErr)
	}
	tmp := filepath.Join(root, "tenant", "uploads", "file.tmp")
	if err = os.WriteFile(tmp, []byte("interrupted"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = driver.ClearTemporary(ctx, profile, "file"); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(tmp); !os.IsNotExist(err) {
		t.Fatalf("temporary object remains: %v", err)
	}
	reader, err = driver.Read(ctx, profile, "file")
	if err != nil {
		t.Fatal(err)
	}
	data, readErr = io.ReadAll(reader)
	closeErr = reader.Close()
	if readErr != nil || closeErr != nil || string(data) != "0123456789" {
		t.Fatalf("final object changed: %q read=%v close=%v", data, readErr, closeErr)
	}
}
func TestFileTypesAreCheckedAgainstBytes(t *testing.T) {
	for _, test := range []struct{ name, purpose, bytes string }{{"fake.png", "logo", "not an image"}, {"fake.pdf", "attachment", "not pdf"}, {"fake.docx", "attachment", "PK"}, {"../file.txt", "attachment", "text"}, {"script.html", "attachment", "<script>"}, {"nul.txt", "attachment", "a\x00b"}} {
		if _, _, err := detect(test.name, test.purpose, []byte(test.bytes)); err == nil {
			t.Fatal("type spoof accepted", test.name)
		}
	}
}
