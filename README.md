# dbkit

A CLI tool for managing PostgreSQL database migrations and seed data. Wraps [goose](https://github.com/pressly/goose) for migrations and provides utilities for creating and applying seed data.

## Installation

```bash
go install github.com/lucasefe/dbkit/cmd/dbkit@latest
```

## Requirements

The following tools must be installed and available in your PATH:

- [goose](https://github.com/pressly/goose) - Database migrations (`go install github.com/pressly/goose/v3/cmd/goose@latest`)
- `psql` / `pg_dump` - PostgreSQL client tools
- `git` - For the check command (CI validation)

## Quick Start

```bash
# Set your database URL
export DATABASE_URL="postgres://user:pass@localhost/mydb"

# Create your first migration
dbkit migrate create create_users_table

# Edit the migration file, then run it
dbkit migrate up

# Check migration status
dbkit migrate status
```

## Integrating dbkit into Your Project

### 1. Project Structure

Set up your project with the following structure:

```
your-project/
├── migrations/           # Migration files go here
│   └── 20240101120000_initial.sql
├── seed/                 # Seed data CSV files
│   ├── public.users.csv
│   └── public.accounts.csv
├── seed.sql              # SQL to select seed data from production
├── Makefile              # Optional: wrap dbkit commands
└── ...
```

### 2. Environment Variables

Configure dbkit using environment variables (12-factor style):

```bash
# Required
export DATABASE_URL="postgres://user:pass@localhost/mydb"

# Optional (with defaults)
export MIGRATIONS_DIR="./migrations"    # default: ./migrations
export SEED_DIR="./seed"                # default: ./seed
export SEED_QUERY_FILE="./seed.sql"     # default: ./seed.sql
```

### 3. Makefile Integration

Add these targets to your `Makefile`:

```makefile
# Database migrations
.PHONY: migrate migrate-down migrate-status migrate-create

migrate:
	dbkit migrate up

migrate-down:
	dbkit migrate down

migrate-status:
	dbkit migrate status

migrate-create:
	@read -p "Migration name: " name; \
	dbkit migrate create $$name

# Seed data
.PHONY: seed seed-create

seed:
	dbkit seed apply

seed-create:
	dbkit seed create -d "$$PROD_DATABASE_URL"

# CI checks
.PHONY: check-migrations

check-migrations:
	dbkit check --base-branch main
```

### 4. CI/CD Integration

Add migration validation to your CI pipeline:

```yaml
# .github/workflows/ci.yml
jobs:
  check-migrations:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0  # Required for branch comparison

      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'

      - name: Install dbkit
        run: go install github.com/lucasefe/dbkit/cmd/dbkit@latest

      - name: Check migration timestamps
        run: dbkit check --base-branch ${{ github.base_ref || 'main' }}
```

## Commands

### migrate

Run database migrations using goose.

```bash
# Run all pending migrations
dbkit migrate up

# Run a single migration
dbkit migrate up-by-one

# Rollback the last migration
dbkit migrate down

# Show migration status
dbkit migrate status

# Create a new migration file
dbkit migrate create add_users_table
# Creates: migrations/20240101120000_add_users_table.sql
```

### seed apply

Apply seed data to your local database. This is useful for setting up development environments.

```bash
dbkit seed apply
```

The apply process:
1. Runs the initial migration (first migration file)
2. Loads all CSV files from the seed directory
3. Runs remaining migrations

### seed create

Create seed data from a database (typically production). This dumps schema and data.

```bash
# Create seed from production
dbkit seed create -d "$PROD_DATABASE_URL"

# Dry run (preview without modifying files)
dbkit seed create -d "$PROD_DATABASE_URL" --dry-run
```

The create process:
1. Flattens all migrations into a single initial migration
2. Exports data to CSV files based on your seed query file

### flatten

Consolidate all migrations into a single initial migration. Useful for cleaning up migration history.

```bash
dbkit flatten -d "$PROD_DATABASE_URL"
```

### check

Validate that new migrations have the latest timestamps. This prevents merge conflicts when multiple developers add migrations.

```bash
dbkit check --base-branch main
```

If validation fails, dbkit provides fix commands:

```
Error: New migrations must have the latest timestamps

To fix:

  $ git mv migrations/{20240101120000,$(date -u +%Y%m%d%H%M%S)}_add_users.sql
```

## Writing Migrations

Migration files use the standard goose format:

```sql
-- +goose Up
-- +goose StatementBegin
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    email TEXT UNIQUE NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE users;
-- +goose StatementEnd
```

## Writing Seed Query Files

The seed query file (`seed.sql`) defines which data to include in your seed. It populates temporary tables that get exported to CSV.

Each table in your database has a corresponding temp table with the naming convention `pg_temp."seed.<schema>.<table>"`.

Example `seed.sql`:

```sql
-- Select recent users for development
INSERT INTO pg_temp."seed.public.users" (id, name, email, created_at)
SELECT id, name, email, created_at
FROM public.users
WHERE created_at > NOW() - INTERVAL '30 days'
LIMIT 100;

-- Select accounts for those users
INSERT INTO pg_temp."seed.public.accounts" (id, user_id, name, balance)
SELECT a.id, a.user_id, a.name, a.balance
FROM public.accounts a
WHERE a.user_id IN (SELECT id FROM pg_temp."seed.public.users");

-- Select related data
INSERT INTO pg_temp."seed.public.transactions" (id, account_id, amount, created_at)
SELECT t.id, t.account_id, t.amount, t.created_at
FROM public.transactions t
WHERE t.account_id IN (SELECT id FROM pg_temp."seed.public.accounts")
LIMIT 1000;
```

## CLI Reference

### Global Flags

```
-d, --database-url string     Database URL (overrides DATABASE_URL env)
-m, --migrations-dir string   Migrations directory (overrides MIGRATIONS_DIR env)
-v, --verbose                 Verbose output
```

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `DATABASE_URL` | PostgreSQL connection URL | required |
| `MIGRATIONS_DIR` | Path to migrations directory | `./migrations` |
| `SEED_DIR` | Path to seed data directory | `./seed` |
| `SEED_QUERY_FILE` | SQL file defining seed queries | `./seed.sql` |

## Examples

### Full Development Setup

```bash
# Clone your project
git clone https://github.com/yourorg/yourproject
cd yourproject

# Set up local database
createdb myproject_dev

# Configure environment
export DATABASE_URL="postgres://localhost/myproject_dev"

# Apply seed data (runs migrations + loads seed data)
dbkit seed apply

# Your database is now ready for development!
```

### Creating New Seed Data

```bash
# Connect to production (read-only)
export PROD_DATABASE_URL="postgres://readonly:pass@prod-host/myproject"

# Create seed from production
dbkit seed create -d "$PROD_DATABASE_URL"

# Review and commit the changes
git add migrations/ seed/
git commit -m "Update seed data"
```

### Adding a New Migration

```bash
# Create migration
dbkit migrate create add_orders_table

# Edit the file
vim migrations/20240101120000_add_orders_table.sql

# Run it
dbkit migrate up

# Commit
git add migrations/
git commit -m "Add orders table"
```

## License

MIT
