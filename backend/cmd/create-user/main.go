// create-user provisions an account. Set DATABASE_URL, NEW_USERNAME,
// NEW_PASSWORD, and NEW_ROLE (admin, promoter, or member) in the environment.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"schemeapp/backend/internal/postgres"
	"schemeapp/backend/internal/security"

	"github.com/joho/godotenv"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if len(os.Args) > 1 {
		if len(os.Args) != 2 || os.Args[1] != "hash-password" {
			fmt.Fprintln(os.Stderr, "usage: create-user [hash-password]")
			os.Exit(2)
		}
		fmt.Fprintln(os.Stderr, "Enter password:")
		if err := printPasswordHash(os.Stdin, os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	err := godotenv.Load()
	if err != nil {
		logger.Error("No .env file found", "error", err)
		os.Exit(1)
	}
	username := strings.TrimSpace(os.Getenv("NEW_USERNAME"))
	password := os.Getenv("NEW_PASSWORD")
	role := strings.TrimSpace(os.Getenv("NEW_ROLE"))
	if username == "" || password == "" || (role != "admin" && role != "promoter" && role != "member") {
		fmt.Fprintln(os.Stderr, "set NEW_USERNAME, NEW_PASSWORD, and NEW_ROLE (admin|promoter|member)")
		os.Exit(2)
	}
	hash, err := security.HashPassword(password)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	db, err := postgres.Open(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer db.Close()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	var userID string
	if err = tx.QueryRowContext(ctx, `INSERT INTO app_users(username,password_hash,role,company_role_id) VALUES($1,$2,$3,CASE WHEN $3='admin' THEN (SELECT id FROM company_roles WHERE role_key='company_admin') ELSE NULL END) RETURNING id::text`, username, hash, role).Scan(&userID); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO audit_entries(actor_user_id,action,entity_type,entity_id,after_state,metadata) VALUES(NULL,'user_bootstrapped','app_users',$1,jsonb_build_object('username',$2::text,'role',$3::text),jsonb_build_object('bootstrap',true))`, userID, username, role); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO event_outbox(event_type,aggregate_type,aggregate_id,payload) VALUES('identity.user_bootstrapped.v1','app_users',$1,jsonb_build_object('username',$2::text,'role',$3::text))`, userID, username, role); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err = tx.Commit(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("User created with ID %s. Link this account to its promoter/member record from the admin master screen.\n", userID)
}
