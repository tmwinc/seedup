package cli

import (
	"context"
	"fmt"

	"github.com/tmwinc/seedup/pkg/executor"
	"github.com/tmwinc/seedup/pkg/seed"
	"github.com/spf13/cobra"
)

var (
	seedDir       string
	seedQueryFile string
	dryRun        bool
)

func newSeedCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "seed",
		Short: "Seed data management commands",
		Long:  "Commands for creating and applying database seed data",
	}

	cmd.AddCommand(newSeedApplyCmd())
	cmd.AddCommand(newSeedCreateCmd())

	return cmd
}

func newSeedApplyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply seed data to the database",
		Long: `Apply seed data to the database.
This runs the initial migration, loads CSV files, then runs remaining migrations.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			dbURL := getDatabaseURL()
			if dbURL == "" {
				return fmt.Errorf("database URL required (use -d flag or DATABASE_URL env)")
			}

			exec := executor.New(executor.WithVerbose(verbose))
			s := seed.New(exec)

			dir := seedDir
			if dir == "" {
				dir = getSeedDir()
			}

			return s.Apply(context.Background(), dbURL, getMigrationsDir(), dir)
		},
	}

	cmd.Flags().StringVar(&seedDir, "seed-dir", "", "Seed data directory (or SEED_DIR env)")

	return cmd
}

func newSeedCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create seed data from a database",
		Long: `Create seed data from a database.
This dumps the schema, flattens migrations, and exports seed data to CSV files.

The seed query file should contain SQL that populates temporary tables with the
data you want to include in the seed. Each table in the database has a corresponding
temp table prefixed with "seed." that you should INSERT INTO.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			dbURL := getDatabaseURL()
			if dbURL == "" {
				return fmt.Errorf("database URL required (use -d flag or DATABASE_URL env)")
			}

			exec := executor.New(executor.WithVerbose(verbose))
			s := seed.New(exec)

			dir := seedDir
			if dir == "" {
				dir = getSeedDir()
			}

			queryFile := seedQueryFile
			if queryFile == "" {
				queryFile = getSeedQueryFile()
			}

			opts := seed.CreateOptions{
				DryRun: dryRun,
			}

			return s.Create(context.Background(), dbURL, getMigrationsDir(), dir, queryFile, opts)
		},
	}

	cmd.Flags().StringVar(&seedDir, "seed-dir", "", "Seed data output directory (or SEED_DIR env)")
	cmd.Flags().StringVar(&seedQueryFile, "seed-query-file", "", "SQL file to select seed data (or SEED_QUERY_FILE env)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview without modifying files")

	return cmd
}
