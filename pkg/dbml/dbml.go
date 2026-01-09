package dbml

import (
	"context"
	"net/url"
	"strings"

	"github.com/tmwinc/seedup/pkg/executor"
)

// Generator handles DBML generation from PostgreSQL databases
type Generator struct {
	exec executor.Executor
}

// Options configures DBML generation
type Options struct {
	Output        string   // Output file (empty = stdout)
	Schemas       []string // Schemas to include (empty = default)
	ExcludeTables []string // Tables to exclude
	AllSchemas    bool     // Include all non-system schemas
}

// New creates a new Generator with the given executor
func New(exec executor.Executor) *Generator {
	return &Generator{exec: exec}
}

// Generate creates a DBML file from the database schema
func (g *Generator) Generate(ctx context.Context, dbURL string, opts Options) error {
	// Ensure sslmode is set (lib/pq requires explicit SSL config unlike psql)
	dbURL = ensureSSLMode(dbURL)

	args := []string{"--url", dbURL}

	if opts.Output != "" {
		args = append(args, "--output", opts.Output)
	}
	if opts.AllSchemas {
		args = append(args, "--all-schemas")
	} else if len(opts.Schemas) > 0 {
		args = append(args, "--schemas", strings.Join(opts.Schemas, ","))
	}
	if len(opts.ExcludeTables) > 0 {
		args = append(args, "--exclude-tables", strings.Join(opts.ExcludeTables, ","))
	}

	return g.exec.Run(ctx, "dbml", args...)
}

// ensureSSLMode adds sslmode=disable if no sslmode is specified in the URL.
// This is needed because lib/pq (used by dbml) requires explicit SSL config,
// unlike psql which defaults to "prefer" and gracefully falls back.
func ensureSSLMode(dbURL string) string {
	u, err := url.Parse(dbURL)
	if err != nil {
		return dbURL
	}

	q := u.Query()
	if q.Get("sslmode") == "" {
		q.Set("sslmode", "disable")
		u.RawQuery = q.Encode()
		return u.String()
	}

	return dbURL
}
