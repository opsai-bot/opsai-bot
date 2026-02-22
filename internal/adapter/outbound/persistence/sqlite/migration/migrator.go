package migration

import (
	"database/sql"
	"embed"
	"fmt"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Run executes all embedded SQL migration files in lexicographic order.
func Run(db *sql.DB) error {
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("reading migrations: %w", err)
	}
	for _, entry := range entries {
		data, err := migrationsFS.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return fmt.Errorf("reading %s: %w", entry.Name(), err)
		}
		if _, err := db.Exec(string(data)); err != nil {
			return fmt.Errorf("executing %s: %w", entry.Name(), err)
		}
	}
	return nil
}
