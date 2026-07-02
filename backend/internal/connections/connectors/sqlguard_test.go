package connectors

import "testing"

func TestEnsureReadOnlySQL(t *testing.T) {
	cases := []struct {
		name    string
		sql     string
		wantErr bool
	}{
		{"simple select", "SELECT * FROM users", false},
		{"cte", "WITH recent AS (SELECT 1) SELECT * FROM recent", false},
		{"trailing semicolon ok", "SELECT 1;", false},
		{"empty", "", true},
		{"insert", "INSERT INTO users (email) VALUES ('x')", true},
		{"update", "UPDATE users SET email = 'x'", true},
		{"delete", "DELETE FROM users", true},
		{"drop", "DROP TABLE users", true},
		{"stacked statements", "SELECT 1; DROP TABLE users;", true},
		{"non select prefix", "EXPLAIN SELECT 1", true},
		{"column named created_at is fine", "SELECT created_at FROM users", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := EnsureReadOnlySQL(tc.sql)
			if tc.wantErr && err == nil {
				t.Fatalf("expected error for %q, got nil", tc.sql)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("expected no error for %q, got %v", tc.sql, err)
			}
		})
	}
}
