package cli

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/peterbourgon/ff/v4"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/internal/cli/normalizedsn"
	"github.com/xo/dburl"
)

// gooseDrivers maps dialects to the driver names used by the goose CLI.
//
// See the ./cmd/goose directory for the driver imports, which are optionally conditionally compiled
// based on build tags.
var gooseDrivers = map[goose.Dialect]string{
	goose.DialectPostgres:   "pgx",
	goose.DialectRedshift:   "pgx",
	goose.DialectMySQL:      "mysql",
	goose.DialectTiDB:       "mysql",
	goose.DialectSQLite3:    "sqlite",
	goose.DialectMSSQL:      "sqlserver",
	goose.DialectClickHouse: "clickhouse",
	goose.DialectVertica:    "vertica",
}

// Usage: goose [OPTIONS] DRIVER DBSTRING COMMAND
// OR
// Usage: goose [OPTIONS] COMMAND

// goose [golbalflags] <command> [flags] [args...]

// openConnection opens a database connection using the given database string.
//
// The database string is parsed using the dburl package.
func openConnection(dbstring string) (*sql.DB, goose.Dialect, error) {
	dbURL, err := dburl.Parse(dbstring)
	if err != nil {
		return nil, "", fmt.Errorf("failed to parse DSN: %w", err)
	}
	dialect, err := resolveDialect(dbURL.UnaliasedDriver)
	if err != nil {
		return nil, "", fmt.Errorf("failed to resolve dialect: %w", err)
	}
	var dataSourceName string
	switch dialect {
	case goose.DialectMySQL:
		dataSourceName, err = normalizedsn.DBString(dataSourceName)
		if err != nil {
			return nil, "", fmt.Errorf("failed to normalize DSN: %w", err)
		}
	default:
		dataSourceName = dbURL.DSN
	}
	// The driver name is used by the goose CLI to open the database connection. It is specific to
	// the goose CLI and the driver imports in ./cmd/goose.
	driverName, ok := gooseDrivers[dialect]
	if !ok {
		return nil, "", fmt.Errorf("unknown database dialect: %s", dialect)
	}
	db, err := sql.Open(driverName, dataSourceName)
	if err != nil {
		return nil, "", fmt.Errorf("failed to open connection: %w", err)
	}
	return db, dialect, nil
}

// resolveDialect returns the dialect for the given string.
//
// The string can be a schema name or an alias. The aliases are defined by the dburl package for
// common databases. See: https://github.com/xo/dburl#database-schemes-aliases-and-drivers
func resolveDialect(s string) (goose.Dialect, error) {
	switch s {
	case "postgres", "pg", "pgx", "postgresql", "pgsql":
		return goose.DialectPostgres, nil
	case "mysql", "my", "mariadb", "maria", "percona", "aurora":
		return goose.DialectMySQL, nil
	case "sqlite", "sqlite3":
		return goose.DialectSQLite3, nil
	case "sqlserver", "ms", "mssql", "azuresql":
		return goose.DialectMSSQL, nil
	case "redshift", "rs":
		return goose.DialectRedshift, nil
	case "tidb", "ti":
		return goose.DialectTiDB, nil
	case "clickhouse", "ch":
		return goose.DialectClickHouse, nil
	case "vertica", "ve":
		return goose.DialectVertica, nil
	default:
		return "", fmt.Errorf("unknown dialect: %q", s)
	}
}

func run(ctx context.Context, state *state, args []string) error {
	root := &rootCmd{
		flags: ff.NewFlagSet("goose"),
	}
	root.flags.BoolVar(&root.version, 0, "version", "print version and exit")

	state.root = root

	root.command = &ff.Command{
		Name:  "goose",
		Flags: root.flags,
		Exec: func(ctx context.Context, args []string) error {
			if root.version {
				fmt.Fprintf(state.stdout, "goose version: %s\n", getVersionFromBuildInfo())
				return nil
			}
			return nil
		},
		ShortHelp: "goose is a database migration tool.",
		Usage:     "goose [globalflags] <subcommand> [flags]",
		LongHelp:  `... long help ...`,
	}
	// Add subcommands.
	commands := []*ff.Command{
		newStatusCommand(state),
	}
	for _, cmd := range commands {
		root.command.Subcommands = append(root.command.Subcommands, cmd)
	}
	// Parse the flags and return help if requested.
	if err := root.command.Parse(args); err != nil {
		if errors.Is(err, ff.ErrHelp) {
			fmt.Fprintf(state.stderr, "\n%s\n", createHelp(root.command, false))
			return nil
		}
		return err
	}
	return root.command.Run(ctx)
}
