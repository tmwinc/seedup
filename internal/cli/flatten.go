package cli

import (
	"context"
	"fmt"

	"github.com/lucasefe/dbkit/pkg/executor"
	"github.com/lucasefe/dbkit/pkg/migrate"
	"github.com/spf13/cobra"
)

func newFlattenCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "flatten",
		Short: "Flatten migrations into a single initial migration",
		Long: `Flatten all applied migrations into a single initial migration.
This dumps the current schema and replaces all migration files with a single file.

This is useful for:
- Reducing the number of migration files in a project
- Creating a clean starting point for new environments
- Simplifying migration history`,
		RunE: func(cmd *cobra.Command, args []string) error {
			dbURL := getDatabaseURL()
			if dbURL == "" {
				return fmt.Errorf("database URL required (use -d flag or DATABASE_URL env)")
			}

			exec := executor.New(executor.WithVerbose(verbose))
			f := migrate.NewFlattener(exec)

			fmt.Println("Flattening migrations...")
			if err := f.Flatten(context.Background(), dbURL, getMigrationsDir()); err != nil {
				return err
			}

			fmt.Println("Migrations flattened successfully")
			return nil
		},
	}
}
