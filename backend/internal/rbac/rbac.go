// Package rbac defines the role-based access control model: a Principal
// (the authenticated caller) carries a resolved, flattened set of permission
// codes, and callers check them with a single Has call. Permissions are
// resolved once at login/refresh time and embedded in the JWT so that
// authorization on every subsequent request is an in-memory set lookup, not
// a database join.
//
// Alongside that account-wide grant, a Principal can also carry
// folder-scoped grants (FolderGrants) - permissions bound to a specific
// folder (and, by construction, its descendants) rather than the whole
// account, resolved the same way (once at login/refresh, embedded in the
// JWT) so folder scoping never adds a database hit to the hot path either.
// HasScoped checks both: the fast, common-case global set first, falling
// back to the (usually empty) folder-scoped map only when needed.
package rbac

import "context"

const (
	PermUsersRead           = "users:read"
	PermUsersWrite          = "users:write"
	PermRolesRead           = "roles:read"
	PermRolesWrite          = "roles:write"
	PermConnectionsRead     = "connections:read"
	PermConnectionsWrite    = "connections:write"
	PermConnectionsTest     = "connections:test"
	PermWorkflowsRead       = "workflows:read"
	PermWorkflowsWrite      = "workflows:write"
	PermWorkflowsExecute    = "workflows:execute"
	PermAuditRead           = "audit:read"
	PermFoldersRead         = "folders:read"
	PermFoldersWrite        = "folders:write"
	PermFoldersManageAccess = "folders:manage_access"
)

// FolderGrant is one folder-scoped grant: userID holds Permissions within
// FolderID (and its descendants). It's the shape both the JWT claim and the
// DB resolution query produce - one entry per folder the user has any
// scoped binding on, already flattened to permission codes.
type FolderGrant struct {
	FolderID    string
	Permissions []string
}

// Principal represents the authenticated caller for the lifetime of a request.
type Principal struct {
	UserID      string
	Email       string
	Roles       []string
	Permissions map[string]struct{}
	// FolderGrants maps folder id -> set of permission codes granted
	// specifically within that folder (and its descendants). Empty for the
	// common case of a principal with only account-wide roles.
	FolderGrants map[string]map[string]struct{}
}

func NewPrincipal(userID, email string, roles []string, permissions []string, folderGrants []FolderGrant) Principal {
	p := Principal{
		UserID:       userID,
		Email:        email,
		Roles:        roles,
		Permissions:  make(map[string]struct{}, len(permissions)),
		FolderGrants: make(map[string]map[string]struct{}, len(folderGrants)),
	}
	for _, perm := range permissions {
		p.Permissions[perm] = struct{}{}
	}
	for _, g := range folderGrants {
		codes := make(map[string]struct{}, len(g.Permissions))
		for _, perm := range g.Permissions {
			codes[perm] = struct{}{}
		}
		p.FolderGrants[g.FolderID] = codes
	}
	return p
}

func (p Principal) Has(permission string) bool {
	_, ok := p.Permissions[permission]
	return ok
}

// HasScoped reports whether the principal holds permission globally, or via
// a folder-scoped grant on any folder in folderChain - typically a target
// folder's own id followed by its ancestor ids (see
// folders.Service.ScopeChain), so a grant on a parent folder covers every
// descendant automatically.
func (p Principal) HasScoped(permission string, folderChain []string) bool {
	if p.Has(permission) {
		return true
	}
	for _, folderID := range folderChain {
		if codes, ok := p.FolderGrants[folderID]; ok {
			if _, ok := codes[permission]; ok {
				return true
			}
		}
	}
	return false
}

// GrantedFolderIDs reports which folders grant permission via a scoped
// binding, for filtering a list query down to what the caller can actually
// see. global=true means the principal holds permission account-wide - the
// caller should skip folder filtering entirely rather than treat an empty
// ids slice as "no access."
func (p Principal) GrantedFolderIDs(permission string) (ids []string, global bool) {
	if p.Has(permission) {
		return nil, true
	}
	for folderID, codes := range p.FolderGrants {
		if _, ok := codes[permission]; ok {
			ids = append(ids, folderID)
		}
	}
	return ids, false
}

func (p Principal) PermissionList() []string {
	out := make([]string, 0, len(p.Permissions))
	for perm := range p.Permissions {
		out = append(out, perm)
	}
	return out
}

// FolderGrantMap renders FolderGrants as a plain folder id -> permission
// codes map, for serializing to the frontend (see handlers.Me) - the
// frontend's own hasScopedPermission check mirrors HasScoped against this.
func (p Principal) FolderGrantMap() map[string][]string {
	out := make(map[string][]string, len(p.FolderGrants))
	for folderID, codes := range p.FolderGrants {
		perms := make([]string, 0, len(codes))
		for perm := range codes {
			perms = append(perms, perm)
		}
		out[folderID] = perms
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
