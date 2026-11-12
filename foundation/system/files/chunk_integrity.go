package files

import (
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"hash"
	"io"
)

// CloudDriveChunkLimit bounds the decoded content of one upload chunk.
const CloudDriveChunkLimit int64 = 4 << 20

var ErrChunkIntegrity = errors.New("file chunk integrity check failed")

// VerifyChunk copies a bounded chunk while checking its length and digest.
// The destination must be temporary: it can contain partial or invalid data
// when this function returns an error and must not be published in that case.
func VerifyChunk(dst io.Writer, src io.Reader, size int64, expected [sha256.Size]byte) error {
	if size < 0 || size > CloudDriveChunkLimit {
		return ErrChunkIntegrity
	}
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(dst, h), io.LimitReader(src, size+1))
	if err != nil {
		return err
	}
	if n != size || subtle.ConstantTimeCompare(h.Sum(nil), expected[:]) != 1 {
		return ErrChunkIntegrity
	}
	return nil
}

// FileDigest incrementally computes the length and digest of a complete file.
// A fresh instance must be used for each assembly attempt.
type FileDigest struct {
	hash hash.Hash
	size int64
}

func NewFileDigest() *FileDigest { return &FileDigest{hash: sha256.New()} }

func (d *FileDigest) Write(p []byte) (int, error) {
	n, err := d.hash.Write(p)
	d.size += int64(n)
	return n, err
}

func (d *FileDigest) Size() int64 { return d.size }

func (d *FileDigest) Sum() [sha256.Size]byte {
	var sum [sha256.Size]byte
	copy(sum[:], d.hash.Sum(nil))
	return sum
}
