package drive

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"horizon/foundation/system/files"
	"horizon/internal/repository/dbgen"
)

func TestAssemblyBoundaryAndMissingChunk(t *testing.T) {
	chunks := make([]dbgen.DriveUploadChunksForAssemblyRow, 256)
	for i := range chunks {
		chunks[i] = dbgen.DriveUploadChunksForAssemblyRow{ChunkIndex: int32(i), ExpectedSize: files.CloudDriveChunkLimit,
			Sha256: pgtype.Text{String: strings.Repeat("0", 64), Valid: true}, UploadedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true}}
	}
	if err := validateAssembly(chunks, maxUploadSize); err != nil {
		t.Fatal(err)
	}
	if err := validateAssembly(chunks[:255], maxUploadSize); err == nil {
		t.Fatal("missing final chunk accepted")
	}
	if err := validateAssembly(chunks, maxUploadSize+1); err == nil {
		t.Fatal("oversized upload accepted")
	}
}

func TestAssemblyRejectsCorruptionAndCanRetry(t *testing.T) {
	ctx := context.Background()
	driver := files.Local{Root: t.TempDir()}
	profile := files.Profile{Driver: "local", Directory: "tenant/uploads"}
	if err := driver.Write(ctx, profile, "chunk", strings.NewReader("bad!")); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte("good"))
	chunks := []dbgen.DriveUploadChunksForAssemblyRow{{ChunkIndex: 0, StorageKey: "chunk", ExpectedSize: 4,
		Sha256: pgtype.Text{String: hex.EncodeToString(sum[:]), Valid: true}, UploadedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true}}}
	if _, err := assembleObject(ctx, driver, profile, "final", chunks, 4); err == nil {
		t.Fatal("corrupted chunk accepted")
	}
	if r, err := driver.Read(ctx, profile, "final"); !errors.Is(err, os.ErrNotExist) {
		if err == nil {
			_ = r.Close()
		}
		t.Fatalf("invalid final object exists: %v", err)
	}
	if err := driver.Delete(ctx, profile, "chunk"); err != nil {
		t.Fatal(err)
	}
	if err := driver.Write(ctx, profile, "chunk", strings.NewReader("good")); err != nil {
		t.Fatal(err)
	}
	got, err := assembleObject(ctx, driver, profile, "final", chunks, 4)
	if err != nil || got != sum {
		t.Fatalf("retry: %x %v", got, err)
	}
	got, err = assembleObject(ctx, driver, profile, "final", chunks, 4)
	if err != nil || got != sum {
		t.Fatalf("existing object replay: %x %v", got, err)
	}
}
