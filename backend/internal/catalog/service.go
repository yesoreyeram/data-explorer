package catalog

import (
	"sort"
	"strings"
)

// Service searches the static seed list. There is no repository/database
// backing this - see the package doc for why.
type Service struct {
	entries []Entry
}

func NewService() *Service {
	entries := append([]Entry(nil), seed...)
	sort.SliceStable(entries, func(i, j int) bool {
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})
	return &Service{entries: entries}
}

// Search returns entries matching all of the given (optional) filters:
// q against name/description (case-insensitive substring), and
// exact matches on category/connType when non-empty.
func (s *Service) Search(q, category, connType string) []Entry {
	q = strings.ToLower(strings.TrimSpace(q))
	category = strings.TrimSpace(category)
	connType = strings.TrimSpace(connType)

	out := make([]Entry, 0, len(s.entries))
	for _, e := range s.entries {
		if q != "" && !strings.Contains(strings.ToLower(e.Name), q) &&
			!strings.Contains(strings.ToLower(e.Description), q) {
			continue
		}
		if category != "" && !strings.EqualFold(e.Category, category) {
			continue
		}
		if connType != "" && string(e.Type) != connType {
			continue
		}
		out = append(out, e)
	}
	return out
}
