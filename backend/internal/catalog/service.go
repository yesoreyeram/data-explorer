package catalog

import "strings"

// Service searches the static seed list. There is no repository/database
// backing this - see the package doc for why.
type Service struct {
	entries []Entry
}

func NewService() *Service {
	return &Service{entries: seed}
}

// Search returns entries matching all of the given (optional) filters:
// q against name/description/category (case-insensitive substring), and
// exact matches on category/connType when non-empty.
func (s *Service) Search(q, category, connType string) []Entry {
	q = strings.ToLower(strings.TrimSpace(q))
	category = strings.TrimSpace(category)
	connType = strings.TrimSpace(connType)

	out := make([]Entry, 0, len(s.entries))
	for _, e := range s.entries {
		if q != "" && !strings.Contains(strings.ToLower(e.Name), q) &&
			!strings.Contains(strings.ToLower(e.Description), q) &&
			!strings.Contains(strings.ToLower(e.Category), q) {
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
