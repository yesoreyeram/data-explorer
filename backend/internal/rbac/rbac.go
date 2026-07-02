// Package rbac defines the role-based access control model: a Principal
// (the authenticated caller) carries a resolved, flattened set of permission
// codes, and callers check them with a single Has call. Permissions are
// resolved once at login/refresh time and embedded in the JWT so that
// authorization on every subsequent request is an in-memory set lookup, not
// a database join.
package rbac

import "context"

const (
	PermUsersRead        = "users:read"
	PermUsersWrite       = "users:write"
	PermRolesRead        = "roles:read"
	PermRolesWrite       = "roles:write"
	PermConnectionsRead  = "connections:read"
	PermConnectionsWrite = "connections:write"
	PermConnectionsTest  = "connections:test"
	PermWorkflowsRead    = "workflows:read"
	PermWorkflowsWrite   = "workflows:write"
	PermWorkflowsExecute = "workflows:execute"
	PermAuditRead        = "audit:read"
)

// Principal represents the authenticated caller for the lifetime of a request.
type Principal struct {
	UserID      string
	Email       string
	Roles       []string
	Permissions map[string]struct{}
}

func NewPrincipal(userID, email string, roles []string, permissions []string) Principal {
	p := Principal{UserID: userID, Email: email, Roles: roles, Permissions: make(map[string]struct{}, len(permissions))}
	for _, perm := range permissions {
		p.Permissions[perm] = struct{}{}
	}
	return p
}

func (p Principal) Has(permission string) bool {
	_, ok := p.Permissions[permission]
	return ok
}

func (p Principal) PermissionList() []string {
	out := make([]string, 0, len(p.Permissions))
	for perm := range p.Permissions {
		out = append(out, perm)
	}
	return out
}

type ctxKey struct{}

func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, ctxKey{}, p)
}

// FromContext returns the caller's Principal and whether one was present
// (i.e. the request passed through the authentication middleware).
func FromContext(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(ctxKey{}).(Principal)
	return p, ok
}
