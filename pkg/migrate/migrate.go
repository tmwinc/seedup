package migrate

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/lucasefe/dbkit/pkg/executor"
)

// MigrationStatus represents the status of a single migration
type MigrationStatus struct {
	Version   string
	Name      string
	Applied   bool
	AppliedAt *time.Time
}

// Migrator handles database migrations using goose
type Migrator struct {
	exec executor.Executor
}

// New creates a new Migrator with the given executor
func New(exec executor.Executor) *Migrator {
	return &Migrator{exec: exec}
}

// Up runs all pending migrations
func (m *Migrator) Up(ctx context.Context, dbURL, migrationsDir string) error {
	return m.exec.Run(ctx, "goose", "postgres", dbURL, "-dir", migrationsDir, "up", "sql")
}

// UpByOne runs a single pending migration
func (m *Migrator) UpByOne(ctx context.Context, dbURL, migrationsDir string) error {
	return m.exec.Run(ctx, "goose", "postgres", dbURL, "-dir", migrationsDir, "up-by-one", "sql")
}

// Down rolls back the last migration
func (m *Migrator) Down(ctx context.Context, dbURL, migrationsDir string) error {
	return m.exec.Run(ctx, "goose", "postgres", dbURL, "-dir", migrationsDir, "down", "sql")
}

// Status shows the status of all migrations
func (m *Migrator) Status(ctx context.Context, dbURL, migrationsDir string) error {
	return m.exec.Run(ctx, "goose", "postgres", dbURL, "-dir", migrationsDir, "status", "sql")
}

// Create creates a new migration file with the given name
func (m *Migrator) Create(migrationsDir, name string) (string, error) {
	timestamp := time.Now().UTC().Format("20060102150405")
	filename := fmt.Sprintf("%s_%s.sql", timestamp, name)
	filepath := filepath.Join(migrationsDir, filename)

	content := `-- +goose Up
-- +goose StatementBegin

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- +goose StatementEnd
`

	if err := os.MkdirAll(migrationsDir, 0755); err != nil {
		return "", fmt.Errorf("creating migrations directory: %w", err)
	}

	if err := os.WriteFile(filepath, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("writing migration file: %w", err)
	}

	return filepath, nil
}
