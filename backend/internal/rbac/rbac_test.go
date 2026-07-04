package rbac

import (
	"reflect"
	"sort"
	"testing"
)

func TestHasScopedGlobalPermissionShortCircuits(t *testing.T) {
	p := NewPrincipal("u1", "u1@example.com", []string{"editor"}, []string{PermConnectionsRead}, nil)
	if !p.HasScoped(PermConnectionsRead, []string{"folder-x"}) {
		t.Fatal("expected global permission to satisfy HasScoped regardless of folder chain")
	}
	if p.HasScoped(PermConnectionsWrite, []string{"folder-x"}) {
		t.Fatal("expected missing permission to fail even with a folder chain")
	}
}

func TestHasScopedFolderGrant(t *testing.T) {
	p := NewPrincipal("u1", "u1@example.com", nil, nil, []FolderGrant{
		{FolderID: "team-a", Permissions: []string{PermConnectionsWrite}},
	})
	if !p.HasScoped(PermConnectionsWrite, []string{"team-a"}) {
		t.Fatal("expected scoped grant on team-a to satisfy HasScoped when team-a is in the chain")
	}
	if p.HasScoped(PermConnectionsWrite, []string{"team-b"}) {
		t.Fatal("expected scoped grant on team-a to NOT satisfy a check against an unrelated folder")
	}
}

func TestHasScopedFolderGrantViaAncestor(t *testing.T) {
	// A grant on "team-a" should cover a subfolder "team-a/sub" when the
	// caller passes the subfolder's full scope chain (self + ancestors).
	p := NewPrincipal("u1", "u1@example.com", nil, nil, []FolderGrant{
		{FolderID: "team-a", Permissions: []string{PermConnectionsWrite}},
	})
	scopeChainForSubfolder := []string{"team-a-sub", "team-a"}
	if !p.HasScoped(PermConnectionsWrite, scopeChainForSubfolder) {
		t.Fatal("expected a grant on an ancestor folder to cover the descendant via its scope chain")
	}
}

func TestHasScopedNoGrantsAtAll(t *testing.T) {
	p := NewPrincipal("u1", "u1@example.com", nil, nil, nil)
	if p.HasScoped(PermConnectionsRead, []string{"any-folder"}) {
		t.Fatal("expected no permission with zero global or folder grants")
	}
}

func TestGrantedFolderIDsGlobalShortCircuits(t *testing.T) {
	p := NewPrincipal("u1", "u1@example.com", []string{"admin"}, []string{PermConnectionsRead}, []FolderGrant{
		{FolderID: "team-a", Permissions: []string{PermConnectionsRead}},
	})
	ids, global := p.GrantedFolderIDs(PermConnectionsRead)
	if !global {
		t.Fatal("expected global=true when the principal holds the permission account-wide")
	}
	if ids != nil {
		t.Fatalf("expected nil ids when global, got %v", ids)
	}
}

func TestGrantedFolderIDsScopedOnly(t *testing.T) {
	p := NewPrincipal("u1", "u1@example.com", nil, nil, []FolderGrant{
		{FolderID: "team-a", Permissions: []string{PermConnectionsRead}},
		{FolderID: "team-b", Permissions: []string{PermConnectionsWrite}},
	})
	ids, global := p.GrantedFolderIDs(PermConnectionsRead)
	if global {
		t.Fatal("expected global=false for a scoped-only principal")
	}
	sort.Strings(ids)
	if !reflect.DeepEqual(ids, []string{"team-a"}) {
		t.Fatalf("expected only team-a to grant connections:read, got %v", ids)
	}
}

func TestGrantedFolderIDsNoAccess(t *testing.T) {
	p := NewPrincipal("u1", "u1@example.com", nil, nil, nil)
	ids, global := p.GrantedFolderIDs(PermConnectionsRead)
	if global || ids != nil {
		t.Fatalf("expected (nil, false) with no access at all, got (%v, %v)", ids, global)
	}
}

func TestFolderGrantMapRoundTrip(t *testing.T) {
	p := NewPrincipal("u1", "u1@example.com", nil, nil, []FolderGrant{
		{FolderID: "team-a", Permissions: []string{PermConnectionsRead, PermConnectionsWrite}},
	})
	m := p.FolderGrantMap()
	sort.Strings(m["team-a"])
	if !reflect.DeepEqual(m["team-a"], []string{PermConnectionsRead, PermConnectionsWrite}) {
		t.Fatalf("unexpected FolderGrantMap output: %v", m)
	}
}
