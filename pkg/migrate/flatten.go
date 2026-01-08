package migrate

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lucasefe/dbkit/pkg/executor"
)

// Flattener consolidates migrations into a single initial migration
type Flattener struct {
	exec executor.Executor
}

// NewFlattener creates a new Flattener with the given executor
func NewFlattener(exec executor.Executor) *Flattener {
	return &Flattener{exec: exec}
}

// Flatten consolidates all applied migrations into a single initial migration
// It dumps the current schema and replaces all migration files with a single initial file
func (f *Flattener) Flatten(ctx context.Context, dbURL, migrationsDir string) error {
	// Get all applied migration versions
	versions, err := f.getAppliedVersions(ctx, dbURL)
	if err != nil {
		return fmt.Errorf("getting applied versions: %w", err)
	}

	if len(versions) == 0 {
		return fmt.Errorf("no applied migrations found")
	}

	// Get the latest version for the new initial migration
	latestVersion := versions[len(versions)-1]

	// Dump the current schema
	schema, err := f.dumpSchema(ctx, dbURL)
	if err != nil {
		return fmt.Errorf("dumping schema: %w", err)
	}

	// Delete all existing migration files
	for _, version := range versions {
		pattern := filepath.Join(migrationsDir, version+"_*.sql")
		matches, _ := filepath.Glob(pattern)
		for _, match := range matches {
			if err := os.Remove(match); err != nil {
				return fmt.Errorf("removing migration file %s: %w", match, err)
			}
		}
	}

	// Create the new initial migration
	initialPath := filepath.Join(migrationsDir, latestVersion+"_initial.sql")
	if err := f.writeInitialMigration(initialPath, schema); err != nil {
		return fmt.Errorf("writing initial migration: %w", err)
	}

	return nil
}

func (f *Flattener) getAppliedVersions(ctx context.Context, dbURL string) ([]string, error) {
	output, err := f.exec.RunSQL(ctx, dbURL,
		"SELECT version_id FROM goose_db_version WHERE is_applied ORDER BY version_id")
	if err != nil {
		return nil, err
	}

	var versions []string
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			versions = append(versions, line)
		}
	}

	return versions, nil
}

func (f *Flattener) dumpSchema(ctx context.Context, dbURL string) (string, error) {
	output, err := f.exec.RunWithOutput(ctx, "pg_dump", dbURL,
		"--schema-only",
		"--no-owner",
		"--exclude-table=public.goose_db_version",
		"--exclude-table=public.goose_db_version_id_seq",
		"--no-privileges",
	)
	if err != nil {
		return "", err
	}

	// Clean up the schema dump
	// Remove search_path config that can cause issues
	// Remove CloudSQL specific commands
	var cleanedLines []string
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(line, "set_config") && strings.Contains(line, "search_path") {
			continue
		}
		if strings.HasPrefix(line, "\\restrict") || strings.HasPrefix(line, "\\unrestrict") {
			continue
		}
		cleanedLines = append(cleanedLines, line)
	}

	return strings.Join(cleanedLines, "\n"), nil
}

func (f *Flattener) writeInitialMigration(path, schema string) error {
	var buf bytes.Buffer

	buf.WriteString("-- +goose Up\n")
	buf.WriteString("-- +goose StatementBegin\n")
	buf.WriteString(schema)
	buf.WriteString("\n-- +goose StatementEnd\n")

	return os.WriteFile(path, buf.Bytes(), 0644)
}
