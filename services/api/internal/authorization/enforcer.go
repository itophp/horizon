package authorization

import (
	"baseline/services/api/internal/auth"
	"embed"
	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
	"github.com/casbin/casbin/v2/persist/string-adapter"
)

//go:embed model.conf policy.csv
var files embed.FS

type Authorizer interface {
	Allow(auth.Principal, string, string) (bool, error)
}
type Engine struct{ enforcer *casbin.SyncedEnforcer }

func New() (*Engine, error) {
	conf, err := files.ReadFile("model.conf")
	if err != nil {
		return nil, err
	}
	policy, err := files.ReadFile("policy.csv")
	if err != nil {
		return nil, err
	}
	m, err := model.NewModelFromString(string(conf))
	if err != nil {
		return nil, err
	}
	e, err := casbin.NewSyncedEnforcer(m, stringadapter.NewAdapter(string(policy)))
	if err != nil {
		return nil, err
	}
	return &Engine{e}, nil
}
func (e *Engine) Allow(p auth.Principal, resource, action string) (bool, error) {
	if p.UserID == "" || p.TenantID == "" {
		return false, nil
	}
	return e.enforcer.Enforce(p.UserID, p.TenantID, resource, action)
}
func (e *Engine) Revoke(p auth.Principal) (bool, error) {
	return e.enforcer.RemoveGroupingPolicy(p.UserID, "demo-editor", p.TenantID)
}
