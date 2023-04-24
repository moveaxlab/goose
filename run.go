package goose

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"time"

	"github.com/pressly/goose/v4/internal/sqlparser"
	"go.uber.org/multierr"
)

// MigrationResult is the result of a successful migration operation.
type MigrationResult struct {
	Migration *Migration
	Duration  time.Duration
	Error     error
	Direction string
	Empty     bool
}

func (p *Provider) runMigrations(
	ctx context.Context,
	conn *sql.Conn,
	migrations []*migration,
	direction sqlparser.Direction,
	byOne bool,
) ([]*MigrationResult, error) {
	if len(migrations) == 0 {
		return nil, nil
	}
	apply := []*migration{migrations[0]}
	if !byOne && len(migrations) > 1 {
		apply = append(apply, migrations[1:]...)
	}
	// Lazy parse SQL migrations (if any) in both directions. We do this before running any
	// migrations so that we can fail fast if there are any errors and avoid leaving the database in
	// a partially migrated state.
	if err := parseSQLMigrations(p.opt.Filesystem, p.opt.Debug, apply); err != nil {
		return nil, err
	}

	// Run migrations individually, opening a new transaction for each migration if the migration is
	// safe to run in a transaction.

	results := make([]*MigrationResult, 0, len(apply))
	for _, m := range apply {
		result := &MigrationResult{
			Migration: m.toMigration(),
			Direction: directionToString(direction),
			Empty:     m.isEmpty(direction),
		}

		start := time.Now()
		if err := p.runIndividually(ctx, conn, direction, m); err != nil {
			result.Error = err
			result.Duration = time.Since(start)
			results = append(results, result)
			return results, fmt.Errorf("migration %s failed: %w", filepath.Base(m.source), err)
		}

		result.Duration = time.Since(start)
		results = append(results, result)
	}
	return results, nil
}

func directionToString(direction sqlparser.Direction) string {
	switch direction {
	case sqlparser.DirectionUp:
		return "up"
	case sqlparser.DirectionDown:
		return "down"
	default:
		return "unknown"
	}
}

// runIndividually runs an individual migration, opening a new transaction if the migration is safe
// to run in a transaction. Otherwise, it runs the migration outside of a transaction with the
// supplied connection.
func (p *Provider) runIndividually(
	ctx context.Context,
	conn *sql.Conn,
	direction sqlparser.Direction,
	m *migration,
) error {
	switch m.migrationType {
	case MigrationTypeSQL:
		if m.sqlMigration.useTx {
			return p.runSQLBeginTx(ctx, conn, direction, m)
		} else {
			return p.runSQLNoTx(ctx, conn, direction, m)
		}
	case MigrationTypeGo:
		if m.goMigration.useTx {
			return p.runGoBeginTx(ctx, conn, direction, m)
		} else {
			// bug(mf): this is a potential deadlock scenario. We're running the Go migration with a
			// *sql.DB, but if/when we introduce locking (which will likely use *sql.Conn) AND if
			// the user set max open connections to 1, then this will deadlock.
			//
			// A potential solution is to expose a third Go register function *sql.Conn. Or continue
			// to use *sql.DB, but to use a separate connection pool for Go migrations and document
			// that the user should NOT SET max open connections to 1.
			//
			// In the Provider constructor we can also throw an error when a user set max open
			// connections to 1 and has Go migrations that are registered to run outside of a
			// transaction.
			return p.runGoNoTx(ctx, direction, m)
		}
	default:
		return fmt.Errorf("unknown migration type: %s", m.migrationType)
	}
}

func (p *Provider) beginTx(
	ctx context.Context,
	conn *sql.Conn,
	direction sqlparser.Direction,
	version int64,
	fn func(tx *sql.Tx) error,
) (retErr error) {
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if retErr != nil {
			retErr = multierr.Append(retErr, tx.Rollback())
		}
	}()
	if err := fn(tx); err != nil {
		return err
	}
	if !p.opt.NoVersioning {
		if err := p.store.InsertOrDelete(ctx, tx, direction.ToBool(), version); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (p *Provider) runGoBeginTx(
	ctx context.Context,
	conn *sql.Conn,
	direction sqlparser.Direction,
	m *migration,
) (retErr error) {
	return p.beginTx(ctx, conn, direction, m.version, func(tx *sql.Tx) error {
		fn := m.goMigration.downFn
		if direction == sqlparser.DirectionUp {
			fn = m.goMigration.upFn
		}
		if fn != nil {
			return fn(ctx, tx)
		}
		return nil
	})
}

func (p *Provider) runSQLBeginTx(
	ctx context.Context,
	conn *sql.Conn,
	direction sqlparser.Direction,
	m *migration,
) error {
	return p.beginTx(ctx, conn, direction, m.version, func(tx *sql.Tx) error {
		statements, err := m.getSQLStatements(direction)
		if err != nil {
			return err
		}
		for _, query := range statements {
			if _, err := tx.ExecContext(ctx, query); err != nil {
				return err
			}
		}
		return nil
	})
}

func (p *Provider) runSQLNoTx(
	ctx context.Context,
	conn *sql.Conn,
	direction sqlparser.Direction,
	m *migration,
) error {
	statements, err := m.getSQLStatements(direction)
	if err != nil {
		return err
	}
	for _, query := range statements {
		if _, err := conn.ExecContext(ctx, query); err != nil {
			return err
		}
	}
	if p.opt.NoVersioning {
		return nil
	}
	return p.store.InsertOrDeleteConn(ctx, conn, direction.ToBool(), m.version)
}

func (p *Provider) runGoNoTx(
	ctx context.Context,
	direction sqlparser.Direction,
	m *migration,
) error {
	fn := m.goMigration.downFnNoTx
	if direction == sqlparser.DirectionUp {
		fn = m.goMigration.upFnNoTx
	}
	if fn != nil {
		if err := fn(ctx, p.db); err != nil {
			return err
		}
	}
	if p.opt.NoVersioning {
		return nil
	}
	return p.store.InsertOrDeleteNoTx(ctx, p.db, direction.ToBool(), m.version)
}

func (p *Provider) initialize(ctx context.Context) (*sql.Conn, func() error, error) {
	conn, err := p.db.Conn(ctx)
	if err != nil {
		return nil, nil, err
	}
	switch p.opt.LockMode {
	case LockModeAdvisorySession:
		if err := p.store.LockSession(ctx, conn); err != nil {
			return nil, nil, err
		}
		cleanup := func() error {
			return multierr.Append(p.store.UnlockSession(ctx, conn), conn.Close())
		}
		return conn, cleanup, nil
	case LockModeNone:
		return conn, conn.Close, nil
	default:
		return nil, nil, fmt.Errorf("invalid lock mode: %d", p.opt.LockMode)
	}
}
