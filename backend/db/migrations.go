// Package db embeds the SQL migration files into the compiled binary so the
// server can migrate its own schema on startup with no separate migration
// tool or file deployment step required.
package db

import "embed"

//go:embed migrations/*.sql
var MigrationsFS embed.FS
