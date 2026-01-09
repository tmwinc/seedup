package db

import (
	"context"
	"fmt"
	"strings"
)

// Drop drops the database specified in the DATABASE_URL
func (m *Manager) Drop(ctx context.Context, dbURL string) error {
	cfg, err := ParseDatabaseURL(dbURL)
	if err != nil {
		return err
	}

	query := fmt.Sprintf("DROP DATABASE IF EXISTS %s WITH (FORCE)", quoteIdent(cfg.Database))
	_, err = m.exec.RunSQL(ctx, cfg.AdminURL(), query)
	if err != nil {
		return fmt.Errorf("dropping database: %w", err)
	}

	return nil
}

// Create creates the database specified in the DATABASE_URL
func (m *Manager) Create(ctx context.Context, dbURL string) error {
	cfg, err := ParseDatabaseURL(dbURL)
	if err != nil {
		return err
	}

	// Check if database already exists
	checkQuery := fmt.Sprintf("SELECT 1 FROM pg_database WHERE datname = '%s'", cfg.Database)
	result, err := m.exec.RunSQL(ctx, cfg.AdminURL(), checkQuery)
	if err != nil {
		return fmt.Errorf("checking database existence: %w", err)
	}

	if strings.TrimSpace(result) == "1" {
		return nil // Database already exists
	}

	query := fmt.Sprintf("CREATE DATABASE %s", quoteIdent(cfg.Database))
	_, err = m.exec.RunSQL(ctx, cfg.AdminURL(), query)
	if err != nil {
		return fmt.Errorf("creating database: %w", err)
	}

	return nil
}

// CreateUser creates the database user if it doesn't exist
func (m *Manager) CreateUser(ctx context.Context, dbURL string) error {
	cfg, err := ParseDatabaseURL(dbURL)
	if err != nil {
		return err
	}

	// Check if user already exists
	checkQuery := fmt.Sprintf("SELECT 1 FROM pg_roles WHERE rolname = '%s'", cfg.User)
	result, err := m.exec.RunSQL(ctx, cfg.AdminURL(), checkQuery)
	if err != nil {
		return fmt.Errorf("checking user existence: %w", err)
	}

	if strings.TrimSpace(result) == "1" {
		return nil // User already exists
	}

	query := fmt.Sprintf("CREATE USER %s WITH PASSWORD '%s'", quoteIdent(cfg.User), cfg.Password)
	_, err = m.exec.RunSQL(ctx, cfg.AdminURL(), query)
	if err != nil {
		return fmt.Errorf("creating user: %w", err)
	}

	return nil
}

// SetupPermissions grants necessary permissions on the database
func (m *Manager) SetupPermissions(ctx context.Context, dbURL string) error {
	cfg, err := ParseDatabaseURL(dbURL)
	if err != nil {
		return err
	}

	// Grant all on the database
	grantDBQuery := fmt.Sprintf("GRANT ALL PRIVILEGES ON DATABASE %s TO %s",
		quoteIdent(cfg.Database), quoteIdent(cfg.User))
	if _, err := m.exec.RunSQL(ctx, cfg.AdminURL(), grantDBQuery); err != nil {
		return fmt.Errorf("granting database privileges: %w", err)
	}

	// Make user owner of the database
	ownerQuery := fmt.Sprintf("ALTER DATABASE %s OWNER TO %s",
		quoteIdent(cfg.Database), quoteIdent(cfg.User))
	if _, err := m.exec.RunSQL(ctx, cfg.AdminURL(), ownerQuery); err != nil {
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

	// 1. Drop database if exists
	fmt.Printf("Dropping database '%s' if exists...\n", cfg.Database)
	if err := m.Drop(ctx, opts.DatabaseURL); err != nil {
		return fmt.Errorf("dropping database: %w", err)
	}

	// 2. Create user if not exists
	fmt.Printf("Creating user '%s' if not exists...\n", cfg.User)
	if err := m.CreateUser(ctx, opts.DatabaseURL); err != nil {
		return fmt.Errorf("creating user: %w", err)
	}

	// 3. Create database
	fmt.Printf("Creating database '%s'...\n", cfg.Database)
	if err := m.Create(ctx, opts.DatabaseURL); err != nil {
		return fmt.Errorf("creating database: %w", err)
	}

	// 4. Setup permissions
	fmt.Println("Setting up permissions...")
	if err := m.SetupPermissions(ctx, opts.DatabaseURL); err != nil {
		return fmt.Errorf("setting up permissions: %w", err)
	}

	// 5. Run migrations
	fmt.Println("Running migrations...")
	if err := m.migrator.Up(ctx, opts.DatabaseURL, opts.MigrationsDir); err != nil {
		return fmt.Errorf("running migrations: %w", err)
	}

	// 6. Apply seeds (optional)
	if !opts.SkipSeed {
		fmt.Println("Applying seeds...")
		if err := m.seeder.Apply(ctx, opts.DatabaseURL, opts.MigrationsDir, opts.SeedDir); err != nil {
			return fmt.Errorf("applying seeds: %w", err)
		}
	}

	return nil
}

// quoteIdent quotes a PostgreSQL identifier to prevent SQL injection
func quoteIdent(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}
