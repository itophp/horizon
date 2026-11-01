package files

import (
	"baseline/services/api/internal/governance"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Driver is the extension boundary. Only compiled and registered drivers are selectable.
// Storage profiles are immutable and passed to every operation, including reads of old files.
type DriverDescriptor struct {
	Fields   []governance.Definition
	Writable bool
}
type Driver interface {
	Descriptor() DriverDescriptor
	Validate(context.Context, Profile) error
	Write(context.Context, Profile, string, io.Reader) error
	Read(context.Context, Profile, string) (io.ReadCloser, error)
	Delete(context.Context, Profile, string) error
	Probe(context.Context, Profile) error
}
type Profile struct {
	Driver, Directory, BaseURL string
	Parameters                 map[string]string
}
type Registry struct {
	Core    *governance.Core
	mu      sync.RWMutex
	drivers map[string]Driver
}

func NewRegistry(root string) *Registry {
	return &Registry{drivers: map[string]Driver{"local": Local{Root: root}}}
}
func (r *Registry) Register(code string, d Driver) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if code == "" || d == nil || r.drivers[code] != nil {
		return governance.Problem("conflict")
	}
	if r.Core == nil {
		return governance.Problem("unavailable")
	}
	seen := map[string]bool{}
	for _, field := range d.Descriptor().Fields {
		if !strings.HasPrefix(field.Key, "storage."+code+".") || r.Core.Definition(field.Key) != nil || seen[field.Key] {
			return governance.Problem("invalid")
		}
		seen[field.Key] = true
	}
	for _, field := range d.Descriptor().Fields {
		field.Scopes = []string{"workspace"}
		field.Sensitive = true
		r.Core.Definitions = append(r.Core.Definitions, field)
	}
	r.Core.Definition("storage.driver").Choices = append(r.Core.Definition("storage.driver").Choices, code)
	r.drivers[code] = d
	return nil
}
func (r *Registry) Get(code string) (Driver, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	d := r.drivers[code]
	if d == nil {
		return nil, governance.Problem("invalid")
	}
	return d, nil
}

type Local struct{ Root string }

func (l Local) Descriptor() DriverDescriptor {
	return DriverDescriptor{Fields: []governance.Definition{}, Writable: true}
}
func (l Local) Validate(_ context.Context, p Profile) error {
	if l.Root == "" || !filepath.IsAbs(l.Root) {
		return governance.Problem("unavailable")
	}
	if !filepath.IsLocal(p.Directory) || strings.ContainsAny(p.Directory, "\\\x00") || p.Driver != "local" {
		return governance.Problem("invalid")
	}
	return nil
}
func (l Local) open(ctx context.Context, p Profile) (*os.Root, error) {
	if err := l.Validate(ctx, p); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(l.Root, 0700); err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(l.Root)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	if err = root.MkdirAll(p.Directory, 0700); err != nil {
		return nil, err
	}
	return root.OpenRoot(p.Directory)
}
func (l Local) Write(ctx context.Context, p Profile, key string, reader io.Reader) error {
	if !filepath.IsLocal(key) || strings.ContainsAny(key, "/\\") {
		return governance.Problem("invalid")
	}
	root, err := l.open(ctx, p)
	if err != nil {
		return err
	}
	defer root.Close()
	file, err := root.OpenFile(key+".tmp", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer func() {
		if e := root.Remove(key + ".tmp"); e != nil && !errors.Is(e, os.ErrNotExist) { // A failed upload cleanup must remain visible to operators.
			governance.LogStorageFailure(e)
		}
	}()
	if _, err = io.Copy(file, reader); err != nil {
		closeErr := file.Close()
		if closeErr != nil {
			governance.LogStorageFailure(closeErr)
		}
		return err
	}
	if err = file.Sync(); err != nil {
		closeErr := file.Close()
		if closeErr != nil {
			governance.LogStorageFailure(closeErr)
		}
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	return root.Rename(key+".tmp", key)
}
func (l Local) Read(ctx context.Context, p Profile, key string) (io.ReadCloser, error) {
	if !filepath.IsLocal(key) || strings.ContainsAny(key, "/\\") {
		return nil, governance.Problem("invalid")
	}
	root, err := l.open(ctx, p)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	return root.Open(key)
}
func (l Local) Delete(ctx context.Context, p Profile, key string) error {
	if !filepath.IsLocal(key) || strings.ContainsAny(key, "/\\") {
		return governance.Problem("invalid")
	}
	root, err := l.open(ctx, p)
	if err != nil {
		return err
	}
	defer root.Close()
	err = root.Remove(key)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
func (l Local) Probe(ctx context.Context, p Profile) error {
	key, err := governance.Token()
	if err != nil {
		return err
	}
	if err = l.Write(ctx, p, key, strings.NewReader("storage-probe")); err != nil {
		return err
	}
	return l.Delete(ctx, p, key)
}
