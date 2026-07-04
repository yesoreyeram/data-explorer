package catalog

import "testing"

func TestSearch(t *testing.T) {
	svc := NewService()

	t.Run("no filters returns everything", func(t *testing.T) {
		got := svc.Search("", "", "")
		if len(got) != len(seed) {
			t.Fatalf("got %d entries, want %d", len(got), len(seed))
		}
	})

	t.Run("query matches name case-insensitively", func(t *testing.T) {
		got := svc.Search("GITHUB", "", "")
		if len(got) != 2 {
			t.Fatalf("got %d entries, want 2 (github-rest, github-graphql)", len(got))
		}
	})

	t.Run("query matches description", func(t *testing.T) {
		got := svc.Search("subscriptions", "", "")
		if len(got) != 1 || got[0].ID != "stripe" {
			t.Fatalf("got %+v, want [stripe]", got)
		}
	})

	t.Run("query does not match category", func(t *testing.T) {
		got := svc.Search("commerce", "", "")
		if len(got) != 0 {
			t.Fatalf("got %d entries, want 0 because q only searches name and description", len(got))
		}
	})

	t.Run("category filter is exact, case-insensitive", func(t *testing.T) {
		got := svc.Search("", "email", "")
		if len(got) != 2 {
			t.Fatalf("got %d entries, want 2 (sendgrid, mailgun)", len(got))
		}
	})

	t.Run("type filter", func(t *testing.T) {
		got := svc.Search("", "", "graphql")
		for _, e := range got {
			if e.Type != "graphql" {
				t.Fatalf("entry %s has type %s, want graphql", e.ID, e.Type)
			}
		}
		if len(got) == 0 {
			t.Fatal("expected at least one graphql entry")
		}
	})

	t.Run("combined filters narrow further", func(t *testing.T) {
		got := svc.Search("shopify", "", "graphql")
		if len(got) != 1 || got[0].ID != "shopify-graphql" {
			t.Fatalf("got %+v, want [shopify-graphql]", got)
		}
	})

	t.Run("no match returns empty, not nil-panicking", func(t *testing.T) {
		got := svc.Search("this-does-not-exist-anywhere", "", "")
		if len(got) != 0 {
			t.Fatalf("got %d entries, want 0", len(got))
		}
	})

	t.Run("results are deterministic by name", func(t *testing.T) {
		got := svc.Search("", "", "")
		for i := 1; i < len(got); i++ {
			if got[i-1].Name > got[i].Name {
				t.Fatalf("entries are not sorted by name: %q before %q", got[i-1].Name, got[i].Name)
			}
		}
	})
}
