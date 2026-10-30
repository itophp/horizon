package governance

import (
	genidentity "baseline/services/api/gen/identity"
	"baseline/services/api/internal/identity"
	"baseline/services/api/internal/repository/dbgen"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"goa.design/goa/v3/security"
	"log/slog"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Error struct{ Code string }

func (e *Error) Error() string  { return e.Code }
func Problem(code string) error { return &Error{code} }
func Code(err error) string {
	var g *Error
	if errors.As(err, &g) {
		return g.Code
	}
	var iam *genidentity.IAMProblem
	if errors.As(err, &iam) {
		return iam.Code
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return "not_found"
	}
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		if pg.Code == "23503" || pg.Code == "23505" {
			return "conflict"
		}
		if pg.Code == "23514" {
			return "invalid"
		}
	}
	slog.Error("governance operation failed", "error_type", fmt.Sprintf("%T", err))
	return "internal"
}

type Options struct {
	EncryptionKey                           []byte
	StorageRoot, PublicWorkspace, PublicURL string
	SMTPHosts                               []string
	AllowTestSMTP                           bool
}
type LogoValidator interface {
	ValidateLogo(context.Context, *identity.Principal, int64, Target) error
	ContextLogo(context.Context, *identity.Principal) (string, string, []byte, error)
}
type StorageValidator interface {
	ValidateStorage(context.Context, string, map[string]Value) error
}
type Core struct {
	Storage     StorageValidator
	Definitions []Definition
	Logos       LogoValidator
	Pool        *pgxpool.Pool
	Q           *dbgen.Queries
	Identity    *identity.Service
	Options     Options
}

func New(pool *pgxpool.Pool, iam *identity.Service, options Options) *Core {
	c := &Core{Definitions: append([]Definition{}, Registry...), Pool: pool, Identity: iam, Options: options}
	if pool != nil {
		c.Q = dbgen.New(pool)
	}
	return c
}
func (c *Core) Available() error {
	if c.Q == nil {
		return Problem("unavailable")
	}
	return nil
}
func (c *Core) APIKeyAuth(ctx context.Context, _ string, _ *security.APIKeyScheme) (context.Context, error) {
	_, err := identity.Current(ctx)
	return ctx, err
}
func Token() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
func Hash(raw string) string                 { sum := sha256.Sum256([]byte(raw)); return hex.EncodeToString(sum[:]) }
func JSON(value interface{}) ([]byte, error) { return json.Marshal(value) }
func Time(value pgtype.Timestamptz) string {
	if !value.Valid {
		return ""
	}
	return value.Time.UTC().Format(time.RFC3339)
}
func Int(v int64) pgtype.Int8 { return pgtype.Int8{Int64: v, Valid: true} }
func (c *Core) Seal(tenant, scope, key, raw string) (string, error) {
	if len(c.Options.EncryptionKey) != 32 {
		return "", Problem("unavailable")
	}
	block, err := aes.NewCipher(c.Options.EncryptionKey)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nonce, nonce, []byte(raw), []byte(tenant+"|"+scope+"|"+key))
	return "enc:v1:" + base64.StdEncoding.EncodeToString(sealed), nil
}
func (c *Core) Unseal(tenant, scope, key, raw string) (string, error) {
	if !strings.HasPrefix(raw, "enc:v1:") || len(c.Options.EncryptionKey) != 32 {
		return "", Problem("unavailable")
	}
	data, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(raw, "enc:v1:"))
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(c.Options.EncryptionKey)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(data) < gcm.NonceSize() {
		return "", Problem("internal")
	}
	value, err := gcm.Open(nil, data[:gcm.NonceSize()], data[gcm.NonceSize():], []byte(tenant+"|"+scope+"|"+key))
	return string(value), err
}

type Target struct {
	Kind, Key string
	ID, OrgID int64
}

func TargetKey(kind string, id int64) string { return kind + ":" + strconv.FormatInt(id, 10) }
func (c *Core) Resolve(ctx context.Context, tenant, kind string, id int64) (Target, error) {
	return c.ResolveWith(ctx, c.Q, tenant, kind, id)
}
func (c *Core) ResolveWith(ctx context.Context, q *dbgen.Queries, tenant, kind string, id int64) (Target, error) {
	t := Target{Kind: kind, ID: id, Key: TargetKey(kind, id)}
	switch kind {
	case "workspace":
		if id != 0 {
			return t, Problem("invalid")
		}
	case "enterprise", "company":
		o, err := q.IAMGetOrganization(ctx, dbgen.IAMGetOrganizationParams{TenantID: tenant, ID: id, AllData: true})
		if err != nil {
			return t, err
		}
		if o.Kind != kind {
			return t, Problem("invalid")
		}
		t.OrgID = id
	case "department":
		d, err := q.IAMGetDepartment(ctx, dbgen.IAMGetDepartmentParams{TenantID: tenant, ID: id, AllData: true})
		if err != nil {
			return t, err
		}
		t.OrgID = d.OrganizationID
	case "account":
		if _, err := q.IAMGetAccount(ctx, dbgen.IAMGetAccountParams{TenantID: tenant, ID: id, AllData: true}); err != nil {
			return t, err
		}
	default:
		return t, Problem("invalid")
	}
	return t, nil
}
func Allowed(scope identity.Scope, t Target) bool {
	return scope.All || (t.Kind == "account" && scope.OwnID == t.ID) || (t.Kind == "company" || t.Kind == "enterprise") && HasInt(scope.OrgIDs, t.ID) || t.Kind == "department" && (HasInt(scope.DeptIDs, t.ID) || HasInt(scope.OrgIDs, t.OrgID))
}
func HasInt(a []int64, v int64) bool {
	for _, x := range a {
		if x == v {
			return true
		}
	}
	return false
}
func (c *Core) Check(ctx context.Context, permission, kind string, id int64, permanent bool) (*identity.Principal, Target, error) {
	if err := c.Available(); err != nil {
		return nil, Target{}, err
	}
	p, scope, err := c.Identity.Authorize(ctx, permission, permanent)
	if err != nil {
		return nil, Target{}, err
	}
	if (strings.HasPrefix(permission, "settings:") || strings.HasPrefix(permission, "dictionaries:") && permission != "dictionaries:use") && (kind != "workspace" || id != 0) {
		return nil, Target{}, Problem("invalid")
	}
	t, err := c.Resolve(ctx, p.TenantID, kind, id)
	if err != nil {
		return nil, t, err
	}
	if !Allowed(scope, t) {
		return nil, t, Problem("forbidden")
	}
	return p, t, nil
}
func (c *Core) EnsureScope(ctx context.Context, q *dbgen.Queries, tenant string, t Target) error {
	p := dbgen.GovEnsureScopeParams{TenantID: tenant, ScopeKey: t.Key, Kind: t.Kind, EntityID: t.ID}
	switch t.Kind {
	case "enterprise", "company":
		p.OrganizationID = Int(t.ID)
	case "department":
		p.DepartmentID = Int(t.ID)
	case "account":
		p.AccountID = Int(t.ID)
	}
	return q.GovEnsureScope(ctx, p)
}
func (c *Core) ScopeTarget(ctx context.Context, tenant, key string) (Target, error) {
	row, err := c.Q.GovGetScope(ctx, dbgen.GovGetScopeParams{TenantID: tenant, ScopeKey: key})
	if err != nil {
		return Target{}, err
	}
	return c.Resolve(ctx, tenant, row.Kind, row.EntityID)
}
func (c *Core) Chain(ctx context.Context, tenant string, t Target, p *identity.Principal) ([]string, error) {
	return c.ChainWith(ctx, c.Q, tenant, t, p)
}
func (c *Core) ChainWith(ctx context.Context, q *dbgen.Queries, tenant string, t Target, p *identity.Principal) ([]string, error) {
	chain := []string{"workspace:0"}
	if t.Kind == "workspace" {
		return chain, nil
	}
	if t.Kind == "account" {
		if p == nil || p.Account.ID != t.ID {
			return nil, Problem("forbidden")
		}
		base, err := c.ResolveWith(ctx, q, tenant, "department", p.Account.DepartmentID)
		if err != nil {
			return nil, err
		}
		chain, err = c.ChainWith(ctx, q, tenant, base, p)
		if err != nil {
			return nil, err
		}
		return append(chain, t.Key), nil
	}
	org := t.OrgID
	if t.Kind == "enterprise" {
		return append(chain, t.Key), nil
	}
	company, err := q.IAMGetOrganization(ctx, dbgen.IAMGetOrganizationParams{TenantID: tenant, ID: org, AllData: true})
	if err != nil {
		return nil, err
	}
	chain = append(chain, TargetKey("enterprise", company.ParentID), TargetKey("company", company.ID))
	if t.Kind == "department" {
		path, err := q.GovDepartmentPath(ctx, dbgen.GovDepartmentPathParams{TenantID: tenant, ID: t.ID})
		if err != nil {
			return nil, err
		}
		for _, d := range path {
			chain = append(chain, TargetKey("department", d.ID))
		}
	}
	return chain, nil
}
func (c *Core) ContextChain(_ context.Context, p *identity.Principal) ([]string, error) {
	if p == nil || p.Account == nil {
		return nil, Problem("forbidden")
	}
	return []string{"workspace:0", TargetKey("account", p.Account.ID)}, nil
}

func (c *Core) AllowedScopes(ctx context.Context, permission string, permanent bool) (*identity.Principal, bool, []string, error) {
	p, scope, err := c.Identity.Authorize(ctx, permission, permanent)
	if err != nil {
		return nil, false, nil, err
	}
	keys, err := c.Q.GovAuthorizedScopes(ctx, dbgen.GovAuthorizedScopesParams{TenantID: p.TenantID, AllData: scope.All, OwnID: scope.OwnID, OrgIds: scope.OrgIDs, DeptIds: scope.DeptIDs})
	return p, scope.All, keys, err
}
func (c *Core) Draft(ctx context.Context, permission string, id int64) (dbgen.GovDraftRow, error) {
	p, all, keys, err := c.AllowedScopes(ctx, permission, true)
	if err != nil {
		return dbgen.GovDraftRow{}, err
	}
	return c.Q.GovDraft(ctx, dbgen.GovDraftParams{TenantID: p.TenantID, ID: id, Column3: all, Column4: keys})
}
func (c *Core) Version(ctx context.Context, permission string, id int64) (dbgen.GovVersionRow, error) {
	p, all, keys, err := c.AllowedScopes(ctx, permission, true)
	if err != nil {
		return dbgen.GovVersionRow{}, err
	}
	return c.Q.GovVersion(ctx, dbgen.GovVersionParams{TenantID: p.TenantID, ID: id, Column3: all, Column4: keys})
}
func (c *Core) Revision(ctx context.Context, tenant, key string) (int64, error) {
	n, err := c.Q.GovRevision(ctx, dbgen.GovRevisionParams{TenantID: tenant, ResourceKey: key})
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	return n, err
}
func (c *Core) Transaction(ctx context.Context, fn func(*dbgen.Queries) error) error {
	tx, err := c.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if e := tx.Rollback(context.WithoutCancel(ctx)); e != nil && !errors.Is(e, pgx.ErrTxClosed) {
			slog.Error("governance rollback failed", "error_type", fmt.Sprintf("%T", e))
		}
	}()
	if err = fn(c.Q.WithTx(tx)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (c *Core) Lock(ctx context.Context, q *dbgen.Queries, p *identity.Principal, t Target, resource string, base int64) error {
	if err := c.EnsureScope(ctx, q, p.TenantID, t); err != nil {
		return err
	}
	if err := q.GovEnsureRevision(ctx, dbgen.GovEnsureRevisionParams{TenantID: p.TenantID, ResourceKey: resource, ScopeKey: t.Key}); err != nil {
		return err
	}
	rev, err := q.GovLockRevision(ctx, dbgen.GovLockRevisionParams{TenantID: p.TenantID, ResourceKey: resource})
	if err != nil {
		return err
	}
	if rev != base {
		return Problem("conflict")
	}
	return nil
}
func (c *Core) CommitVersion(ctx context.Context, q *dbgen.Queries, p *identity.Principal, t Target, domain, resource string, entity, base int64, payload []byte, reason string, detail []byte) (int64, error) {
	rev, err := q.GovBumpRevision(ctx, dbgen.GovBumpRevisionParams{TenantID: p.TenantID, ResourceKey: resource, Revision: base})
	if err != nil {
		return 0, err
	}
	id, err := q.GovAddVersion(ctx, dbgen.GovAddVersionParams{TenantID: p.TenantID, Domain: domain, ResourceKey: resource, ScopeKey: t.Key, EntityID: entity, Revision: rev, Payload: payload, Reason: reason, ActorID: p.Account.ID})
	if err != nil {
		return 0, err
	}
	err = q.GovAudit(ctx, dbgen.GovAuditParams{TenantID: p.TenantID, ScopeKey: t.Key, ActorID: p.Account.ID, Action: domain + ".publish", ResourceKey: resource, Revision: rev, Detail: detail})
	return id, err
}
func (c *Core) Consume(ctx context.Context, q *dbgen.Queries, p *identity.Principal, proof, operation string, id, revision int64) error {
	if proof == "" {
		return Problem("forbidden")
	}
	_, err := q.GovConsumeProof(ctx, dbgen.GovConsumeProofParams{TenantID: p.TenantID, TokenHash: Hash(proof), SessionHash: p.TokenHash, DepartmentID: p.Account.DepartmentID, Operation: operation, ResourceID: id, Revision: revision})
	if err != nil {
		return Problem("forbidden")
	}
	return nil
}

type Value struct {
	Raw, Source string
	Configured  bool
}

func (c *Core) Values(ctx context.Context, tenant string, chain []string) (map[string]Value, error) {
	return c.ValuesWith(ctx, c.Q, tenant, chain)
}
func (c *Core) ValuesWith(ctx context.Context, q *dbgen.Queries, tenant string, chain []string) (map[string]Value, error) {
	rows, err := q.GovValues(ctx, dbgen.GovValuesParams{TenantID: tenant, Column2: chain})
	if err != nil {
		return nil, err
	}
	out := map[string]Value{}
	for _, d := range c.Definitions {
		out[d.Key] = Value{Raw: d.Default, Source: "default"}
	}
	for _, key := range chain {
		for _, row := range rows {
			if row.ScopeKey != key {
				continue
			}
			d := c.Definition(row.ConfigKey)
			if d == nil {
				return nil, Problem("internal")
			}
			raw := string(row.ValueJson)
			if d.Key == "files.maxBytes" || d.Key == "files.logoMaxBytes" {
				old, _ := Number(out[d.Key].Raw)
				next, e := Number(raw)
				if e != nil {
					return nil, e
				}
				if next > old {
					continue
				}
			}
			if d.Key == "files.extensions" {
				var prev, next []string
				if err = json.Unmarshal([]byte(out[d.Key].Raw), &prev); err != nil {
					return nil, err
				}
				if err = json.Unmarshal([]byte(raw), &next); err != nil {
					return nil, err
				}
				filtered := []string{}
				for _, x := range next {
					if Has(prev, x) {
						filtered = append(filtered, x)
					}
				}
				encoded, e := json.Marshal(filtered)
				if e != nil {
					return nil, e
				}
				raw = string(encoded)
			}
			out[row.ConfigKey] = Value{Raw: raw, Source: key, Configured: raw != `""`}
		}
	}
	return out, nil
}
func String(raw string) (string, error) {
	if strings.TrimSpace(raw) == "null" {
		return "", Problem("invalid")
	}
	var v string
	err := json.Unmarshal([]byte(raw), &v)
	return v, err
}
func Number(raw string) (int64, error) {
	if strings.TrimSpace(raw) == "null" {
		return 0, Problem("invalid")
	}
	var v int64
	err := json.Unmarshal([]byte(raw), &v)
	return v, err
}
func Bool(raw string) (bool, error) {
	if strings.TrimSpace(raw) == "null" {
		return false, Problem("invalid")
	}
	var v bool
	err := json.Unmarshal([]byte(raw), &v)
	return v, err
}
func (c *Core) Setting(ctx context.Context, p *identity.Principal, key string) (string, error) {
	chain, err := c.ContextChain(ctx, p)
	if err != nil {
		return "", err
	}
	values, err := c.Values(ctx, p.TenantID, chain)
	if err != nil {
		return "", err
	}
	v, ok := values[key]
	if !ok {
		return "", Problem("invalid")
	}
	d := c.Definition(key)
	if d.Secret && v.Source != "default" {
		encoded, err := String(v.Raw)
		if err != nil {
			return "", err
		}
		if encoded == "" {
			return `""`, nil
		}
		return c.Unseal(p.TenantID, v.Source, key, encoded)
	}
	return v.Raw, nil
}
func (c *Core) SecurityPolicy(ctx context.Context, tenant string) (identity.SecurityPolicy, error) {
	out := identity.DefaultSecurityPolicy()
	if c.Q == nil {
		return out, Problem("unavailable")
	}
	v, err := c.Values(ctx, tenant, []string{"workspace:0"})
	if err != nil {
		return out, err
	}
	numbers := map[string]*int{"security.passwordMin": &out.PasswordMin, "security.lockAttempts": &out.LockAttempts, "security.lockMinutes": &out.LockMinutes, "security.sessionMinutes": &out.SessionMinutes, "security.idleMinutes": &out.IdleMinutes, "temporary.reasonMin": &out.ReasonMin, "temporary.maximumMinutes": &out.MaximumMinutes}
	for k, dest := range numbers {
		n, err := Number(v[k].Raw)
		if err != nil {
			return out, err
		}
		*dest = int(n)
	}
	bools := map[string]*bool{"security.requireUpper": &out.Upper, "security.requireLower": &out.Lower, "security.requireDigit": &out.Digit, "security.requireSymbol": &out.Symbol}
	for k, dest := range bools {
		b, err := Bool(v[k].Raw)
		if err != nil {
			return out, err
		}
		*dest = b
	}
	err = json.Unmarshal([]byte(v["temporary.allowedMinutes"].Raw), &out.AllowedMinutes)
	return out, err
}
func (c *Core) ValidateCombined(values map[string]Value) error {
	idle, err := Number(values["security.idleMinutes"].Raw)
	if err != nil {
		return err
	}
	absolute, err := Number(values["security.sessionMinutes"].Raw)
	if err != nil || idle > absolute {
		return Problem("invalid")
	}
	maximum, err := Number(values["temporary.maximumMinutes"].Raw)
	if err != nil {
		return err
	}
	var minutes []int
	if err = json.Unmarshal([]byte(values["temporary.allowedMinutes"].Raw), &minutes); err != nil {
		return err
	}
	for _, v := range minutes {
		if int64(v) > maximum {
			return Problem("invalid")
		}
	}
	enabled, err := Bool(values["smtp.enabled"].Raw)
	if err != nil {
		return err
	}
	if enabled {
		host, err := String(values["smtp.host"].Raw)
		if err != nil || host == "" || !Has(c.Options.SMTPHosts, host) {
			return Problem("invalid")
		}
		from, err := String(values["smtp.from"].Raw)
		if err != nil || from == "" {
			return Problem("invalid")
		}
	}
	raw, err := String(values["storage.baseUrl"].Raw)
	if err != nil {
		return err
	}
	if raw != "" {
		u, err := url.Parse(raw)
		if err != nil {
			return err
		}
		if u.Scheme == "http" && u.Hostname() != "127.0.0.1" && u.Hostname() != "localhost" && u.Hostname() != "::1" {
			return Problem("invalid")
		}
	}
	return nil
}

func LogStorageFailure(err error) {
	slog.Error("storage cleanup failed", "error_type", fmt.Sprintf("%T", err))
}

func (c *Core) Definition(key string) *Definition {
	for i := range c.Definitions {
		if c.Definitions[i].Key == key {
			return &c.Definitions[i]
		}
	}
	return nil
}

// Audit records only deliberately projected metadata, in the mutation transaction.
func (c *Core) Audit(ctx context.Context, q *dbgen.Queries, p *identity.Principal, t Target, action, resource string, detail []byte) error {
	if err := c.EnsureScope(ctx, q, p.TenantID, t); err != nil {
		return err
	}
	return q.GovAudit(ctx, dbgen.GovAuditParams{TenantID: p.TenantID, ScopeKey: t.Key, ActorID: p.Account.ID, Action: action, ResourceKey: resource, Detail: detail})
}

func (c *Core) WithReadQueries(q *dbgen.Queries) *Core {
	copy := *c
	copy.Q = q
	copy.Identity = c.Identity.WithReadQueries(q)
	return &copy
}
