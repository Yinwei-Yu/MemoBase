package infra

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
)

//go:embed schema.sql
var initSchema string

func NewDB(databaseURL string) (*sqlx.DB, error) {
	db, err := sqlx.Connect("pgx", databaseURL)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(30 * time.Minute)
	return db, nil
}

func InitSchema(ctx context.Context, db *sqlx.DB) error {
	if initSchema == "" {
		return fmt.Errorf("init schema is empty")
	}
	_, err := db.ExecContext(ctx, initSchema)
	if err != nil {
		return err
	}
	return nil
}

func Ping(ctx context.Context, db *sqlx.DB) error {
	var one int
	return db.QueryRowContext(ctx, "SELECT 1").Scan(&one)
}

func IsNotFound(err error) bool {
	return err == sql.ErrNoRows
}
