package seed

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Apply seeds the database with data from CSV files
// It runs the initial migration, loads seed data, then runs remaining migrations
func (s *Seeder) Apply(ctx context.Context, dbURL, migrationsDir, seedDir string) error {
	// Run the initial migration (schema at point of creating seed)
	fmt.Println("Running initial migration...")
	if err := s.migrator.UpByOne(ctx, dbURL, migrationsDir); err != nil {
		return fmt.Errorf("running initial migration: %w", err)
	}

	// Build and execute the seed script
	fmt.Println("Seeding database...")
	if err := s.loadSeedData(ctx, dbURL, seedDir); err != nil {
		return fmt.Errorf("loading seed data: %w", err)
	}

	// Run all remaining migrations
	fmt.Println("Running remaining migrations...")
	if err := s.migrator.Up(ctx, dbURL, migrationsDir); err != nil {
		return fmt.Errorf("running remaining migrations: %w", err)
	}

	return nil
}

func (s *Seeder) loadSeedData(ctx context.Context, dbURL, seedDir string) error {
	csvFiles, err := filepath.Glob(filepath.Join(seedDir, "*.csv"))
	if err != nil {
		return fmt.Errorf("finding CSV files: %w", err)
	}

	if len(csvFiles) == 0 {
		fmt.Println("No CSV files found in seed directory")
		return nil
	}

	var script bytes.Buffer

	// Begin transaction and disable triggers
	script.WriteString("BEGIN;\n")
	script.WriteString("SET session_replication_role = 'replica';\n")

	// Generate COPY commands for each CSV file
	for _, csv := range csvFiles {
		table := strings.TrimSuffix(filepath.Base(csv), ".csv")
		absPath, err := filepath.Abs(csv)
		if err != nil {
			return fmt.Errorf("getting absolute path for %s: %w", csv, err)
		}
		script.WriteString(fmt.Sprintf("\\COPY %s FROM '%s' WITH CSV HEADER;\n", table, absPath))
	}

	// Re-enable triggers
	script.WriteString("SET session_replication_role = 'origin';\n")

	// Force constraint validation by touching each table
	for _, csv := range csvFiles {
		table := strings.TrimSuffix(filepath.Base(csv), ".csv")
		firstColumn, err := s.getFirstColumn(csv)
		if err != nil {
			return fmt.Errorf("getting first column for %s: %w", csv, err)
		}
		script.WriteString(fmt.Sprintf("UPDATE %s SET \"%s\" = \"%s\";\n", table, firstColumn, firstColumn))
	}

	script.WriteString("COMMIT;\n")

	// Write script to temp file and execute
	tmpFile, err := os.CreateTemp("", "seed-*.sql")
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

func (s *Seeder) getFirstColumn(csvPath string) (string, error) {
	data, err := os.ReadFile(csvPath)
	if err != nil {
		return "", err
	}

	lines := strings.Split(string(data), "\n")
	if len(lines) == 0 {
		return "", fmt.Errorf("empty CSV file")
	}

	headers := strings.Split(lines[0], ",")
	if len(headers) == 0 {
		return "", fmt.Errorf("no headers in CSV file")
	}

	return strings.TrimSpace(headers[0]), nil
}
