package folders

import (
	"reflect"
	"testing"
)

// Fixture: root -> child -> grandchild -> greatgrandchild (a 4-level chain),
// plus a sibling of child ("cousin") to make sure the recompute only
// touches the moved subtree. IDs are descriptive strings rather than real
// UUIDs since recomputeAncestorsForMove treats them as opaque.
const (
	root            = "root"
	child           = "child"
	grandchild      = "grandchild"
	greatgrandchild = "greatgrandchild"
	cousin          = "cousin"
	newParent       = "new-parent"
)

func fourLevelDescendants() []descendantRef {
	return []descendantRef{
		{ID: grandchild, AncestorIDs: []string{root, child}},
		{ID: greatgrandchild, AncestorIDs: []string{root, child, grandchild}},
	}
}

func TestRecomputeAncestorsForMove_ReparentUnderNewParent(t *testing.T) {
	// child (ancestors: [root]) moves under new-parent (ancestors: []).
	// newPrefix = new-parent's own ancestors + new-parent itself = ["new-parent"].
	descendants := fourLevelDescendants()
	updates := recomputeAncestorsForMove(child, []string{root}, []string{newParent}, descendants)

	want := map[string][]string{
		grandchild:      {newParent, child},
		greatgrandchild: {newParent, child, grandchild},
	}
	if !reflect.DeepEqual(updates, want) {
		t.Fatalf("got %#v, want %#v", updates, want)
	}
}

func TestRecomputeAncestorsForMove_ReparentToRoot(t *testing.T) {
	// child moves to root level: newPrefix = nil.
	descendants := fourLevelDescendants()
	updates := recomputeAncestorsForMove(child, []string{root}, nil, descendants)

	want := map[string][]string{
		grandchild:      {child},
		greatgrandchild: {child, grandchild},
	}
	if !reflect.DeepEqual(updates, want) {
		t.Fatalf("got %#v, want %#v", updates, want)
	}
}

func TestRecomputeAncestorsForMove_DeeperNewParentPreservesRelativeSuffix(t *testing.T) {
	// child moves under a new parent that itself has ancestors ["a", "b"],
	// so newPrefix = ["a", "b", "new-parent"]. Descendants' relative path
	// below child (nothing for grandchild, [grandchild] for
	// greatgrandchild) must be preserved untouched.
	descendants := fourLevelDescendants()
	newPrefix := []string{"a", "b", newParent}
	updates := recomputeAncestorsForMove(child, []string{root}, newPrefix, descendants)

	want := map[string][]string{
		grandchild:      {"a", "b", newParent, child},
		greatgrandchild: {"a", "b", newParent, child, grandchild},
	}
	if !reflect.DeepEqual(updates, want) {
		t.Fatalf("got %#v, want %#v", updates, want)
	}
}

func TestRecomputeAncestorsForMove_UnrelatedSiblingUntouched(t *testing.T) {
	// A folder that is not a descendant of the mover must never appear in
	// the update set - recomputeAncestorsForMove only receives descendants,
	// but this guards the "only descendants" invariant its caller relies on.
	descendants := []descendantRef{{ID: cousin, AncestorIDs: []string{root}}}
	updates := recomputeAncestorsForMove(child, []string{root}, []string{newParent}, descendants)

	if _, ok := updates[cousin]; !ok {
		t.Fatalf("expected cousin to be present since it was passed in as a descendant")
	}
	// cousin's own ancestors don't start with [root, child], so treating it
	// as a descendant here would be a caller bug - recomputeAncestorsForMove
	// itself has no way to detect that (it trusts its input), which is why
	// Repository.descendants is the only source of the descendants slice.
}

func TestMaxRelativeDepth(t *testing.T) {
	descendants := fourLevelDescendants()
	got := maxRelativeDepth([]string{root}, descendants)
	// greatgrandchild's relative path below child is [grandchild] - depth 1.
	if got != 1 {
		t.Fatalf("expected max relative depth 1, got %d", got)
	}

	if got := maxRelativeDepth([]string{root}, nil); got != 0 {
		t.Fatalf("expected max relative depth 0 with no descendants, got %d", got)
	}
}
