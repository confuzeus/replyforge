package config

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"
)

func RunMigrations(db *sql.DB, migrationsFS embed.FS) error {
	files, err := fs.ReadDir(migrationsFS, ".")
	if err != nil {
		return fmt.Errorf("reading migrations directory: %w", err)
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].Name() < files[j].Name()
	})

	for _, f := range files {
		if f.IsDir() {
			continue
		}

		content, err := fs.ReadFile(migrationsFS, f.Name())
		if err != nil {
			return fmt.Errorf("reading migration %s: %w", f.Name(), err)
		}

		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("starting transaction for %s: %w", f.Name(), err)
		}

		if _, err := tx.Exec(string(content)); err != nil {
			tx.Rollback()
			return fmt.Errorf("executing migration %s: %w", f.Name(), err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("committing migration %s: %w", f.Name(), err)
		}
	}

	return nil
}
