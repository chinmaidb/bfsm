package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		logger.Error("load .env file", "error", err)
		os.Exit(1)
	}
	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseURL == "" {
		logger.Error("DATABASE_URL is required")
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	pgxConfig, err := pgx.ParseConfig(databaseURL)
	if err != nil {
		logger.Error("parse database URL", "error", err)
		os.Exit(1)
	}
	pgxConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	db := stdlib.OpenDB(*pgxConfig)
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		logger.Error("connect to database", "error", err)
		os.Exit(1)
	}
	migrationsDir := strings.TrimSpace(os.Getenv("MIGRATIONS_DIR"))
	if migrationsDir == "" {
		migrationsDir = "migrations"
	}
	if err := migrate(ctx, db, migrationsDir); err != nil {
		logger.Error("database migration failed", "error", err)
		os.Exit(1)
	}
	logger.Info("database migrations are up to date")
}

func migrate(ctx context.Context, db *sql.DB, directory string) error {
	files, err := migrationFiles(directory)
	if err != nil {
		return err
	}

	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version text PRIMARY KEY,
		applied_at timestamptz NOT NULL DEFAULT now()
	)`); err != nil {
		return fmt.Errorf("create schema_migrations table: %w", err)
	}

	for _, file := range files {
		version := filepath.Base(file)
		var applied bool
		err := db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)`, version).Scan(&applied)
		if err != nil {
			return fmt.Errorf("check migration %s: %w", version, err)
		}
		if applied {
			continue
		}
		contents, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", version, err)
		}
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin migration %s: %w", version, err)
		}
		if _, err = tx.ExecContext(ctx, string(contents)); err == nil {
			_, err = tx.ExecContext(ctx, `INSERT INTO schema_migrations(version) VALUES ($1)`, version)
		}
		if err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply migration %s: %w", version, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %s: %w", version, err)
		}
		slog.Info("applied database migration", "version", version)
	}
	return nil
}

func migrationFiles(directory string) ([]string, error) {
	files, err := filepath.Glob(filepath.Join(directory, "*.sql"))
	if err != nil {
		return nil, fmt.Errorf("list migration files: %w", err)
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no SQL migration files found in %q", directory)
	}
	sort.Strings(files)
	return files, nil
}
