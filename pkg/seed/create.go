package seed

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tmwinc/seedup/pkg/migrate"
)

// CreateOptions configures the seed creation process
type CreateOptions struct {
	DryRun bool
}

// Create creates seed data from a database
// It dumps the schema, flattens migrations, and exports seed data to CSV files
func (s *Seeder) Create(ctx context.Context, dbURL, migrationsDir, seedDir, queryFile string, opts CreateOptions) error {
	// Ensure seed directory exists
	if err := os.MkdirAll(seedDir, 0755); err != nil {
		return fmt.Errorf("creating seed directory: %w", err)
	}

	// Get all tables in the database
	tables, err := s.getTables(ctx, dbURL)
	if err != nil {
		return fmt.Errorf("getting tables: %w", err)
	}

	// Build and execute the seed data extraction script
	tempDir, err := os.MkdirTemp("", "dbkit-seed-*")
	if err != nil {
		return fmt.Errorf("creating temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	if err := s.extractSeedData(ctx, dbURL, tables, queryFile, tempDir); err != nil {
		return fmt.Errorf("extracting seed data: %w", err)
	}

	if opts.DryRun {
		fmt.Println("Dry run mode - not modifying any files")
		return nil
	}

	// Flatten migrations
	flattener := migrate.NewFlattener(s.exec)
	if err := flattener.Flatten(ctx, dbURL, migrationsDir); err != nil {
		return fmt.Errorf("flattening migrations: %w", err)
	}

	// Clean old CSV files and move new ones
	oldCSVs, _ := filepath.Glob(filepath.Join(seedDir, "*.csv"))
	for _, csv := range oldCSVs {
		os.Remove(csv)
	}

	newCSVs, _ := filepath.Glob(filepath.Join(tempDir, "*.csv"))
	for _, csv := range newCSVs {
		dest := filepath.Join(seedDir, filepath.Base(csv))
		data, err := os.ReadFile(csv)
		if err != nil {
			return fmt.Errorf("reading %s: %w", csv, err)
		}
		if err := os.WriteFile(dest, data, 0644); err != nil {
			return fmt.Errorf("writing %s: %w", dest, err)
		}
	}

	return nil
}

type tableInfo struct {
	Schema string
	Name   string
}

func (s *Seeder) getTables(ctx context.Context, dbURL string) ([]tableInfo, error) {
	output, err := s.exec.RunSQL(ctx, dbURL, `
		SELECT schemaname, tablename
		FROM pg_catalog.pg_tables
		WHERE schemaname NOT IN ('information_schema', 'pg_catalog')
		AND schemaname NOT LIKE 'pg_temp%'
		AND tablename <> 'goose_db_version'
		ORDER BY schemaname, tablename
	`)
	if err != nil {
		return nil, err
	}

	var tables []tableInfo
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, "|")
		if len(parts) != 2 {
			continue
		}
		tables = append(tables, tableInfo{
			Schema: strings.TrimSpace(parts[0]),
			Name:   strings.TrimSpace(parts[1]),
		})
	}

	return tables, nil
}

func (s *Seeder) extractSeedData(ctx context.Context, dbURL string, tables []tableInfo, queryFile, outputDir string) error {
	var script bytes.Buffer

	// Create temp tables for each real table
	for _, t := range tables {
		tempTable := fmt.Sprintf("pg_temp.\"seed.%s.%s\"", t.Schema, t.Name)
		script.WriteString(fmt.Sprintf("CREATE TEMP TABLE %s (LIKE \"%s\".\"%s\" INCLUDING ALL);\n",
			tempTable, t.Schema, t.Name))
	}

	// Include the user's seed query file which populates the temp tables
	if queryFile != "" {
		queryContent, err := os.ReadFile(queryFile)
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Printf("Warning: seed query file '%s' not found, proceeding without custom queries\n", queryFile)
			} else {
				return fmt.Errorf("reading query file: %w", err)
			}
		} else {
			script.WriteString("\n")
			script.Write(queryContent)
			script.WriteString("\n")
		}
	}

	// Export each temp table to CSV
	for _, t := range tables {
		tempTable := fmt.Sprintf("pg_temp.\"seed.%s.%s\"", t.Schema, t.Name)
		csvPath := filepath.Join(outputDir, fmt.Sprintf("%s.%s.csv", t.Schema, t.Name))
		script.WriteString(fmt.Sprintf("\\copy %s TO '%s' CSV HEADER\n", tempTable, csvPath))
		script.WriteString(fmt.Sprintf("\\echo Exported %s.%s\n", t.Schema, t.Name))
	}

	// Write script to temp file and execute
	tmpFile, err := os.CreateTemp("", "seed-create-*.sql")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(script.String()); err != nil {
		return fmt.Errorf("writing seed script: %w", err)
	}
	tmpFile.Close()

	return s.exec.RunSQLFile(ctx, dbURL, tmpFile.Name())
}
