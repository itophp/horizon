package files

import (
	"archive/zip"
	"baseline/services/api/internal/governance"
	"baseline/services/api/internal/identity"
	"baseline/services/api/internal/repository/dbgen"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

type Service struct {
	*governance.Core
	Storage *Registry
}

func New(c *governance.Core) *Service {
	s := &Service{Core: c, Storage: NewRegistry(c.Options.StorageRoot)}
	s.Storage.Core = c
	c.Logos = s
	c.Storage = s
	return s
}
func apiError(err *error) {
	if *err != nil {
		*err = governance.Problem(governance.Code(*err))
	}
}
func (s *Service) profile(ctx context.Context, tenant string, id int64) (Profile, error) {
	row, err := s.Q.GovStorageProfile(ctx, dbgen.GovStorageProfileParams{TenantID: tenant, ID: id})
	if err != nil {
		return Profile{}, err
	}
	var p Profile
	if err = json.Unmarshal(row.Config, &p); err != nil {
		return p, err
	}
	if p.Driver != row.Driver {
		return p, governance.Problem("internal")
	}
	driver, err := s.Storage.Get(p.Driver)
	if err != nil {
		return p, err
	}
	for _, field := range driver.Descriptor().Fields {
		if field.Secret && p.Parameters[field.Key] != "" {
			raw, e := s.Unseal(tenant, "storage-profile", field.Key, p.Parameters[field.Key])
			if e != nil {
				return p, e
			}
			p.Parameters[field.Key] = raw
		}
	}
	return p, nil
}
func (s *Service) currentProfile(ctx context.Context, p *identity.Principal) (Profile, error) {
	out := Profile{}
	for key, dest := range map[string]*string{"storage.driver": &out.Driver, "storage.directory": &out.Directory, "storage.baseUrl": &out.BaseURL} {
		raw, err := s.Setting(ctx, p, key)
		if err != nil {
			return out, err
		}
		value, err := governance.String(raw)
		if err != nil {
			return out, err
		}
		*dest = value
	}
	out.Parameters = map[string]string{}
	driver, err := s.Storage.Get(out.Driver)
	if err != nil {
		return out, err
	}
	for _, field := range driver.Descriptor().Fields {
		raw, e := s.Setting(ctx, p, field.Key)
		if e != nil {
			return out, e
		}
		out.Parameters[field.Key] = raw
	}
	out.Directory = filepath.Join(governance.Hash(p.TenantID)[:24], out.Directory)
	return out, nil
}
func (s *Service) file(ctx context.Context, p *identity.Principal, scope identity.Scope, id int64) (dbgen.GovFileRow, error) {
	return s.Q.GovFile(ctx, dbgen.GovFileParams{TenantID: p.TenantID, ID: id, AllData: scope.All, OwnID: scope.OwnID, OrgIds: scope.OrgIDs, DeptIds: scope.DeptIDs})
}
func (s *Service) record(ctx context.Context, tenant string, row dbgen.GovFileRow) (*ManagedFile, error) {
	profile, err := s.profile(ctx, tenant, row.StorageProfileID)
	if err != nil {
		return nil, err
	}
	return &ManagedFile{ID: row.ID, Name: row.Name, Mime: row.Mime, Size: row.Size, Purpose: row.Purpose, CreatedAt: governance.Time(row.CreatedAt), OwnerID: row.OwnerID, DepartmentID: row.DepartmentID, StorageVersion: row.StorageProfileID, Link: strings.TrimRight(profile.BaseURL, "/") + "/files/" + strconvID(row.ID)}, nil
}
func strconvID(v int64) string { return strconv.FormatInt(v, 10) }
func (s *Service) List(ctx context.Context, _ *ListPayload) (res []*ManagedFile, err error) {
	defer apiError(&err)
	p, scope, err := s.Identity.Authorize(ctx, "files:read", false)
	if err != nil {
		return nil, err
	}
	rows, err := s.Q.GovFiles(ctx, dbgen.GovFilesParams{TenantID: p.TenantID, AllData: scope.All, OwnID: scope.OwnID, OrgIds: scope.OrgIDs, DeptIds: scope.DeptIDs})
	if err != nil {
		return nil, err
	}
	res = []*ManagedFile{}
	for _, row := range rows {
		r, e := s.record(ctx, p.TenantID, dbgen.GovFileRow(row))
		if e != nil {
			return nil, e
		}
		res = append(res, r)
	}
	return res, nil
}
func (s *Service) Drivers(ctx context.Context, _ *DriversPayload) (res []*StorageDriver, err error) {
	defer apiError(&err)
	if _, _, err = s.Identity.Authorize(ctx, "settings:read", true); err != nil {
		return nil, err
	}
	s.Storage.mu.RLock()
	defer s.Storage.mu.RUnlock()
	res = []*StorageDriver{}
	for code, driver := range s.Storage.drivers {
		keys := []string{"storage.directory", "storage.baseUrl"}
		for _, field := range driver.Descriptor().Fields {
			keys = append(keys, field.Key)
		}
		res = append(res, &StorageDriver{Code: code, Writable: driver.Descriptor().Writable, ConfigKeys: keys})
	}
	sort.Slice(res, func(i, j int) bool { return res[i].Code < res[j].Code })
	return res, nil
}
func detect(name, purpose string, content []byte) (string, []byte, error) {
	if strings.ContainsAny(name, "\x00\r\n/\\") || name == "." || name == ".." {
		return "", nil, governance.Problem("invalid")
	}
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(name)), ".")
	mime := http.DetectContentType(content)
	switch ext {
	case "png", "jpg", "jpeg":
		if ext == "png" && mime != "image/png" || ext != "png" && mime != "image/jpeg" {
			return "", nil, governance.Problem("invalid")
		}
		cfg, _, err := image.DecodeConfig(bytes.NewReader(content))
		if err != nil || cfg.Width <= 0 || cfg.Height <= 0 || int64(cfg.Width)*int64(cfg.Height) > 16000000 {
			return "", nil, governance.Problem("invalid")
		}
		if purpose == "logo" {
			img, _, err := image.Decode(bytes.NewReader(content))
			if err != nil {
				return "", nil, err
			}
			var b bytes.Buffer
			if ext == "png" {
				err = png.Encode(&b, img)
			} else {
				err = jpeg.Encode(&b, img, &jpeg.Options{Quality: 90})
			}
			if err != nil {
				return "", nil, err
			}
			content = b.Bytes()
		}
	case "webp":
		if purpose == "logo" || len(content) < 12 || string(content[:4]) != "RIFF" || string(content[8:12]) != "WEBP" {
			return "", nil, governance.Problem("invalid")
		}
		mime = "image/webp"
	case "pdf":
		if !bytes.HasPrefix(content, []byte("%PDF-")) {
			return "", nil, governance.Problem("invalid")
		}
		mime = "application/pdf"
	case "txt", "csv":
		if !utf8.Valid(content) || bytes.IndexByte(content, 0) >= 0 {
			return "", nil, governance.Problem("invalid")
		}
		mime = "text/plain"
		if ext == "csv" {
			mime = "text/csv"
		}
	case "docx", "xlsx":
		r, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
		if err != nil || len(r.File) > 1000 {
			return "", nil, governance.Problem("invalid")
		}
		contentTypes, document := false, false
		for _, file := range r.File {
			contentTypes = contentTypes || file.Name == "[Content_Types].xml"
			document = document || ext == "docx" && file.Name == "word/document.xml" || ext == "xlsx" && file.Name == "xl/workbook.xml"
		}
		if !contentTypes || !document {
			return "", nil, governance.Problem("invalid")
		}
		if ext == "docx" {
			mime = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
		} else {
			mime = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
		}
	default:
		return "", nil, governance.Problem("invalid")
	}
	if purpose == "logo" && mime != "image/png" && mime != "image/jpeg" {
		return "", nil, governance.Problem("invalid")
	}
	return mime, content, nil
}
func (s *Service) Upload(ctx context.Context, payload *UploadPayload) (res *ManagedFile, err error) {
	defer apiError(&err)
	p, scope, err := s.Identity.Authorize(ctx, "files:create", true)
	if err != nil {
		return nil, err
	}
	if !scope.All && scope.OwnID != p.Account.ID && !governance.HasInt(scope.OrgIDs, p.Account.OrganizationID) && !governance.HasInt(scope.DeptIDs, p.Account.DepartmentID) {
		return nil, governance.Problem("forbidden")
	}
	raw, err := s.Setting(ctx, p, "files.extensions")
	if err != nil {
		return nil, err
	}
	var allowed []string
	if err = json.Unmarshal([]byte(raw), &allowed); err != nil {
		return nil, err
	}
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(payload.Name)), ".")
	if !governance.Has(allowed, ext) {
		return nil, governance.Problem("invalid")
	}
	key := "files.maxBytes"
	if payload.Purpose == "logo" {
		key = "files.logoMaxBytes"
	}
	raw, err = s.Setting(ctx, p, key)
	if err != nil {
		return nil, err
	}
	limit, err := governance.Number(raw)
	if err != nil {
		return nil, err
	}
	if int64(len(payload.Content)) > limit {
		return nil, governance.Problem("invalid")
	}
	mime, data, err := detect(payload.Name, payload.Purpose, payload.Content)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, governance.Problem("invalid")
	}
	profile, err := s.currentProfile(ctx, p)
	if err != nil {
		return nil, err
	}
	driver, err := s.Storage.Get(profile.Driver)
	if err != nil {
		return nil, err
	}
	if err = driver.Validate(ctx, profile); err != nil {
		return nil, err
	}
	configRaw, err := json.Marshal(profile)
	if err != nil {
		return nil, err
	}
	stored := profile
	stored.Parameters = map[string]string{}
	for k, v := range profile.Parameters {
		stored.Parameters[k] = v
	}
	for _, field := range driver.Descriptor().Fields {
		if field.Secret {
			encrypted, e := s.Seal(p.TenantID, "storage-profile", field.Key, stored.Parameters[field.Key])
			if e != nil {
				return nil, e
			}
			stored.Parameters[field.Key] = encrypted
		}
	}
	encoded, err := json.Marshal(stored)
	if err != nil {
		return nil, err
	}
	version, err := s.Q.GovEnsureStorageProfile(ctx, dbgen.GovEnsureStorageProfileParams{TenantID: p.TenantID, Driver: profile.Driver, ConfigHash: governance.Hash(string(configRaw)), Config: encoded})
	if err != nil {
		return nil, err
	}
	storageKey, err := governance.Token()
	if err != nil {
		return nil, err
	}
	if err = driver.Write(ctx, profile, storageKey, bytes.NewReader(data)); err != nil {
		return nil, err
	}
	metadataSaved := false
	defer func() {
		if err != nil && !metadataSaved {
			if e := driver.Delete(context.WithoutCancel(ctx), profile, storageKey); e != nil {
				governance.LogStorageFailure(e)
			}
		}
	}()
	sum := sha256.Sum256(data)
	var id int64
	err = s.Transaction(ctx, func(q *dbgen.Queries) error {
		var e error
		id, e = q.GovCreateFile(ctx, dbgen.GovCreateFileParams{TenantID: p.TenantID, OwnerID: p.Account.ID, OrganizationID: p.Account.OrganizationID, DepartmentID: p.Account.DepartmentID, StorageProfileID: version, StorageKey: storageKey, Name: payload.Name, Mime: mime, Size: int64(len(data)), Purpose: payload.Purpose, Sha256: hex.EncodeToString(sum[:])})
		if e != nil {
			return e
		}
		t := governance.Target{Kind: "department", ID: p.Account.DepartmentID, Key: governance.TargetKey("department", p.Account.DepartmentID)}
		detail, e := json.Marshal(map[string]interface{}{"purpose": payload.Purpose, "size": len(data), "storageVersion": version})
		if e != nil {
			return e
		}
		return s.Audit(ctx, q, p, t, "files.create", "file:"+strconvID(id), detail)
	})
	if err != nil {
		return nil, err
	}
	metadataSaved = true
	row, err := s.file(ctx, p, identity.Scope{TenantID: p.TenantID, OwnID: p.Account.ID}, id)
	if err != nil {
		return nil, err
	}
	return s.record(ctx, p.TenantID, row)
}
func (s *Service) read(ctx context.Context, tenant string, row dbgen.GovFileRow) ([]byte, error) {
	profile, err := s.profile(ctx, tenant, row.StorageProfileID)
	if err != nil {
		return nil, err
	}
	driver, err := s.Storage.Get(profile.Driver)
	if err != nil {
		return nil, err
	}
	reader, err := driver.Read(ctx, profile, row.StorageKey)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	data, err := io.ReadAll(io.LimitReader(reader, 20971521))
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(data)
	if int64(len(data)) != row.Size || hex.EncodeToString(sum[:]) != row.Sha256 {
		return nil, governance.Problem("internal")
	}
	return data, nil
}
func (s *Service) Content(ctx context.Context, payload *ContentPayload) (res *FileContent, err error) {
	defer apiError(&err)
	p, scope, err := s.Identity.Authorize(ctx, "files:read", false)
	if err != nil {
		return nil, err
	}
	row, err := s.file(ctx, p, scope, payload.ID)
	if err != nil {
		return nil, err
	}
	data, err := s.read(ctx, p.TenantID, row)
	if err != nil {
		return nil, err
	}
	return &FileContent{Name: row.Name, Mime: row.Mime, Content: data}, nil
}
func (s *Service) Remove(ctx context.Context, payload *RemovePayload) (err error) {
	defer apiError(&err)
	p, scope, err := s.Identity.Authorize(ctx, "files:delete", true)
	if err != nil {
		return err
	}
	return s.Transaction(ctx, func(q *dbgen.Queries) error {
		if e := q.GovLockAttachment(ctx, dbgen.GovLockAttachmentParams{TenantID: p.TenantID, FileID: payload.ID}); e != nil {
			return e
		}
		if _, e := q.GovDeleteFile(ctx, dbgen.GovDeleteFileParams{TenantID: p.TenantID, ID: payload.ID, AllData: scope.All, OwnID: scope.OwnID, OrgIds: scope.OrgIDs, DeptIds: scope.DeptIDs}); e != nil {
			return e
		}
		t := governance.Target{Kind: "department", ID: p.Account.DepartmentID, Key: governance.TargetKey("department", p.Account.DepartmentID)}
		return s.Audit(ctx, q, p, t, "files.delete", "file:"+strconvID(payload.ID), []byte(`{}`))
	})
}
func (s *Service) Purge(ctx context.Context) error {
	rows, err := s.Q.GovPendingFileDeletes(ctx)
	if err != nil {
		return err
	}
	for _, row := range rows {
		p, e := s.profile(ctx, row.TenantID, row.StorageProfileID)
		if e != nil {
			return e
		}
		driver, e := s.Storage.Get(p.Driver)
		if e != nil {
			return e
		}
		if e = driver.Delete(ctx, p, row.StorageKey); e != nil {
			return e
		}
		if e = s.Q.GovPurgedFile(ctx, dbgen.GovPurgedFileParams{TenantID: row.TenantID, ID: row.ID}); e != nil {
			return e
		}
	}
	return nil
}
func (s *Service) ValidateLogo(ctx context.Context, p *identity.Principal, id int64, _ governance.Target) error {
	scope, err := s.Identity.ScopeFor(ctx, p, "files:read", true)
	if err != nil {
		return err
	}
	row, err := s.file(ctx, p, scope, id)
	if err != nil {
		return err
	}
	if row.Purpose != "logo" || (row.Mime != "image/png" && row.Mime != "image/jpeg") {
		return governance.Problem("invalid")
	}
	return nil
}

type Branding struct{ Files *Service }

func (b *Branding) Get(ctx context.Context) (res *EffectiveConfiguration, err error) {
	defer func() {
		if err != nil {
			err = governance.Problem("internal")
		}
	}()
	values := map[string]governance.Value{}
	for _, d := range b.Files.Definitions {
		values[d.Key] = governance.Value{Raw: d.Default, Source: "default"}
	}
	c := b.Files.Core
	if c.Options.PublicWorkspace != "" && c.Q != nil {
		values, err = c.Values(ctx, c.Options.PublicWorkspace, []string{"workspace:0"})
		if err != nil {
			return nil, err
		}
	}
	entries := []*ConfigEntry{}
	for _, d := range b.Files.Definitions {
		if strings.HasPrefix(d.Key, "branding.") || d.Key == "display.language" || d.Key == "display.theme" {
			v := values[d.Key]
			entries = append(entries, &ConfigEntry{Key: d.Key, Value: v.Raw, Source: "public", Configured: v.Source != "default"})
		}
	}
	encoded, err := json.Marshal(entries)
	if err != nil {
		return nil, err
	}
	return &EffectiveConfiguration{Entries: entries, Revision: governance.Hash(string(encoded))}, nil
}
func (b *Branding) Logo(ctx context.Context) (res *FileContent, err error) {
	defer func() {
		if err != nil {
			err = governance.Problem("not_found")
		}
	}()
	c := b.Files.Core
	if c.Q == nil || c.Options.PublicWorkspace == "" {
		return nil, governance.Problem("not_found")
	}
	values, err := c.Values(ctx, c.Options.PublicWorkspace, []string{"workspace:0"})
	if err != nil {
		return nil, err
	}
	id, err := governance.Number(values["branding.logo"].Raw)
	if err != nil || id <= 0 {
		return nil, governance.Problem("not_found")
	}
	row, err := b.Files.Q.GovFile(ctx, dbgen.GovFileParams{TenantID: c.Options.PublicWorkspace, ID: id, AllData: true})
	if err != nil {
		return nil, err
	}
	if row.Purpose != "logo" {
		return nil, governance.Problem("not_found")
	}
	data, err := b.Files.read(ctx, c.Options.PublicWorkspace, row)
	if err != nil {
		return nil, err
	}
	return &FileContent{Name: row.Name, Mime: row.Mime, Content: data}, nil
}

func (s *Service) ValidateStorage(ctx context.Context, tenant string, values map[string]governance.Value) error {
	p := Profile{Parameters: map[string]string{}}
	for key, dest := range map[string]*string{"storage.driver": &p.Driver, "storage.directory": &p.Directory, "storage.baseUrl": &p.BaseURL} {
		v, e := governance.String(values[key].Raw)
		if e != nil {
			return e
		}
		*dest = v
	}
	driver, e := s.Storage.Get(p.Driver)
	if e != nil {
		return e
	}
	if !driver.Descriptor().Writable {
		return governance.Problem("invalid")
	}
	for _, f := range driver.Descriptor().Fields {
		v := values[f.Key]
		raw := v.Raw
		if f.Secret && v.Source != "default" {
			encoded, e := governance.String(raw)
			if e != nil {
				return e
			}
			if encoded != "" {
				raw, e = s.Unseal(tenant, v.Source, f.Key, encoded)
				if e != nil {
					return e
				}
			}
		}
		p.Parameters[f.Key] = raw
	}
	p.Directory = filepath.Join(governance.Hash(tenant)[:24], p.Directory)
	if e = driver.Validate(ctx, p); e != nil {
		return e
	}
	return driver.Probe(ctx, p)
}

func (s *Service) ContextLogo(ctx context.Context, p *identity.Principal) (string, string, []byte, error) {
	chain, err := s.ContextChain(ctx, p)
	if err != nil {
		return "", "", nil, err
	}
	values, err := s.Values(ctx, p.TenantID, chain)
	if err != nil {
		return "", "", nil, err
	}
	id, err := governance.Number(values["branding.logo"].Raw)
	if err != nil || id <= 0 {
		return "", "", nil, governance.Problem("not_found")
	}
	row, err := s.Q.GovFile(ctx, dbgen.GovFileParams{TenantID: p.TenantID, ID: id, AllData: true})
	if err != nil {
		return "", "", nil, err
	}
	if row.Purpose != "logo" {
		return "", "", nil, governance.Problem("not_found")
	}
	data, err := s.read(ctx, p.TenantID, row)
	return row.Name, row.Mime, data, err
}

// NotificationContent uses recipient binding in SQL; a recipient needs no global files:read grant.
func (s *Service) NotificationContent(ctx context.Context, messageID, fileID int64) (string, string, []byte, error) {
	p, err := identity.Current(ctx)
	if err != nil {
		return "", "", nil, err
	}
	row, err := s.Q.GovNotificationFile(ctx, dbgen.GovNotificationFileParams{TenantID: p.TenantID, ID: messageID, RecipientID: p.Account.ID, ID_2: fileID})
	if err != nil {
		return "", "", nil, err
	}
	data, err := s.read(ctx, p.TenantID, dbgen.GovFileRow(row))
	return row.Name, row.Mime, data, err
}
