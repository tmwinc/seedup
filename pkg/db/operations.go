package db

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Drop drops the database specified in the DATABASE_URL
func (m *Manager) Drop(ctx context.Context, dbURL, adminURL string) error {
	cfg, err := ParseDatabaseURL(dbURL)
	if err != nil {
		return err
	}

	if adminURL == "" {
		adminURL = cfg.AdminURL()
	}

	query := fmt.Sprintf("DROP DATABASE IF EXISTS %s WITH (FORCE)", quoteIdent(cfg.Database))
	_, err = m.exec.RunSQL(ctx, adminURL, query)
	if err != nil {
		return fmt.Errorf("dropping database: %w", err)
	}

	return nil
}

// Create creates the database specified in the DATABASE_URL
func (m *Manager) Create(ctx context.Context, dbURL, adminURL string) error {
	cfg, err := ParseDatabaseURL(dbURL)
	if err != nil {
		return err
	}

	if adminURL == "" {
		adminURL = cfg.AdminURL()
	}

	// Check if database already exists
	checkQuery := fmt.Sprintf("SELECT 1 FROM pg_database WHERE datname = '%s'", cfg.Database)
	result, err := m.exec.RunSQL(ctx, adminURL, checkQuery)
	if err != nil {
		return fmt.Errorf("checking database existence: %w", err)
	}

	if strings.TrimSpace(result) == "1" {
		return nil // Database already exists
	}

	query := fmt.Sprintf("CREATE DATABASE %s", quoteIdent(cfg.Database))
	_, err = m.exec.RunSQL(ctx, adminURL, query)
	if err != nil {
		return fmt.Errorf("creating database: %w", err)
	}

	return nil
}

// CreateUser creates the database user if it doesn't exist
func (m *Manager) CreateUser(ctx context.Context, dbURL, adminURL string) error {
	cfg, err := ParseDatabaseURL(dbURL)
	if err != nil {
		return err
	}

	if adminURL == "" {
		adminURL = cfg.AdminURL()
	}

	// Check if user already exists
	checkQuery := fmt.Sprintf("SELECT 1 FROM pg_roles WHERE rolname = '%s'", cfg.User)
	result, err := m.exec.RunSQL(ctx, adminURL, checkQuery)
	if err != nil {
		return fmt.Errorf("checking user existence: %w", err)
	}

	if strings.TrimSpace(result) == "1" {
		return nil // User already exists
	}

	query := fmt.Sprintf("CREATE USER %s WITH PASSWORD '%s'", quoteIdent(cfg.User), cfg.Password)
	_, err = m.exec.RunSQL(ctx, adminURL, query)
	if err != nil {
		return fmt.Errorf("creating user: %w", err)
	}

	return nil
}

// SetupPermissions grants necessary permissions on the database
func (m *Manager) SetupPermissions(ctx context.Context, dbURL, adminURL string) error {
	cfg, err := ParseDatabaseURL(dbURL)
	if err != nil {
		return err
	}

	if adminURL == "" {
		adminURL = cfg.AdminURL()
	}

	// Grant all on the database
	grantDBQuery := fmt.Sprintf("GRANT ALL PRIVILEGES ON DATABASE %s TO %s",
		quoteIdent(cfg.Database), quoteIdent(cfg.User))
	if _, err := m.exec.RunSQL(ctx, adminURL, grantDBQuery); err != nil {
		return fmt.Errorf("granting database privileges: %w", err)
	}

	// Make user owner of the database
	ownerQuery := fmt.Sprintf("ALTER DATABASE %s OWNER TO %s",
		quoteIdent(cfg.Database), quoteIdent(cfg.User))
	if _, err := m.exec.RunSQL(ctx, adminURL, ownerQuery); err != nil {
		return fmt.Errorf("setting database owner: %w", err)
	}

	return nil
}

// Setup performs a full database setup: drop, create user, create db, permissions, migrate, seed
func (m *Manager) Setup(ctx context.Context, opts SetupOptions) error {
	cfg, err := ParseDatabaseURL(opts.DatabaseURL)
	if err != nil {
		return err
	}

	adminURL := opts.AdminURL
	if adminURL == "" {
		adminURL = cfg.AdminURL()
	}

	// 1. Create user if not exists (do this first so DROP doesn't fail if user doesn't exist)
	fmt.Printf("Creating user '%s' if not exists...\n", cfg.User)
	if err := m.CreateUser(ctx, opts.DatabaseURL, adminURL); err != nil {
		return fmt.Errorf("creating user: %w", err)
	}

	// 2. Drop database if exists
	fmt.Printf("Dropping database '%s' if exists...\n", cfg.Database)
	if err := m.Drop(ctx, opts.DatabaseURL, adminURL); err != nil {
		return fmt.Errorf("dropping database: %w", err)
	}

	// 3. Create database
	fmt.Printf("Creating database '%s'...\n", cfg.Database)
	if err := m.Create(ctx, opts.DatabaseURL, adminURL); err != nil {
		return fmt.Errorf("creating database: %w", err)
	}

	// 4. Setup permissions
	fmt.Println("Setting up permissions...")
	if err := m.SetupPermissions(ctx, opts.DatabaseURL, adminURL); err != nil {
		return fmt.Errorf("setting up permissions: %w", err)
	}

	// 5. Run migrations (if any exist)
	migrations, _ := filepath.Glob(filepath.Join(opts.MigrationsDir, "*.sql"))
	if len(migrations) > 0 {
		fmt.Println("Running migrations...")
		if err := m.migrator.Up(ctx, opts.DatabaseURL, opts.MigrationsDir); err != nil {
			return fmt.Errorf("running migrations: %w", err)
		}
	} else {
		fmt.Println("No migration files found, skipping migrations")
	}

	// 6. Apply seeds (optional, requires --seed-name)
	if !opts.SkipSeed && opts.SeedName != "" {
		seedDir := filepath.Join(opts.SeedDir, opts.SeedName)
		if _, err := os.Stat(seedDir); err == nil {
			csvFiles, _ := filepath.Glob(filepath.Join(seedDir, "*.csv"))
			if len(csvFiles) > 0 {
				fmt.Printf("Applying seeds from '%s'...\n", seedDir)
				if err := m.seeder.Apply(ctx, opts.DatabaseURL, opts.MigrationsDir, seedDir); err != nil {
					return fmt.Errorf("applying seeds: %w", err)
				}
			} else {
				fmt.Printf("No seed CSV files found in '%s', skipping seeds\n", seedDir)
			}
		} else {
			fmt.Printf("Seed directory '%s' not found, skipping seeds\n", seedDir)
		}
	} else if !opts.SkipSeed && opts.SeedName == "" {
		fmt.Println("No --seed-name provided, skipping seeds")
	}

	return nil
}

// quoteIdent quotes a PostgreSQL identifier to prevent SQL injection
func quoteIdent(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}
