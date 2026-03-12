package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"

	"sentinal-chat/pkg/logger"
)

// DBTX abstracts *sql.DB and *sql.Tx.
type DBTX interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}

func buildPlaceholders(start, count int) string {
	if count <= 0 {
		return ""
	}
	parts := make([]string, count)
	for i := range count {
		parts[i] = fmt.Sprintf("$%d", start+i)
	}
	return strings.Join(parts, ",")
}

func logSQLError(l *logger.Logger, operation, query string, args []any, err error) {
	if l == nil || err == nil {
		return
	}
	compactQuery := strings.Join(strings.Fields(query), " ")
	l.Errorf("sql error operation=%s err=%v query=%q args=%v", operation, err, compactQuery, args)
}

// WithTx executes fn inside a transaction when db is *sql.DB.
// If db is already a *sql.Tx, fn is executed directly.
func WithTx(ctx context.Context, db DBTX, fn func(DBTX) error) error {
	if db == nil {
		return errors.New("database not initialized")
	}
	if tx, ok := db.(*sql.Tx); ok {
		return fn(tx)
	}
	sqlDB, ok := db.(*sql.DB)
	if !ok {
		return errors.New("unsupported db type")
	}
	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("tx error: %v (rollback error: %w)", err, rbErr)
		}
		return err
	}
	return tx.Commit()
}
