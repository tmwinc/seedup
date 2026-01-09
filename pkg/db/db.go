package db

import (
	"github.com/tmwinc/seedup/pkg/executor"
	"github.com/tmwinc/seedup/pkg/migrate"
	"github.com/tmwinc/seedup/pkg/seed"
)

// Manager handles database setup operations
type Manager struct {
	exec     executor.Executor
	migrator *migrate.Migrator
	seeder   *seed.Seeder
}

// New creates a new Manager with the given executor
func New(exec executor.Executor) *Manager {
	return &Manager{
		exec:     exec,
		migrator: migrate.New(exec),
		seeder:   seed.New(exec),
	}
}

// SetupOptions configures the Setup operation
type SetupOptions struct {
	DatabaseURL   string
	MigrationsDir string
	SeedDir       string
	SkipSeed      bool
}
