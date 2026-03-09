package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

func GetDB() *sql.DB {
	connSpec := os.Getenv("DINHA_DATABASE_URL")
	if connSpec == "" {
		log.Fatal("DINHA_DATABASE_URL environment variable is required")
	}

	// Expand ~ to home directory
	if strings.HasPrefix(connSpec, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			log.Fatalf("failed to get home directory: %v", err)
		}
		connSpec = filepath.Join(home, connSpec[1:])
	}

	// Ensure parent directory exists
	dir := filepath.Dir(connSpec)
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Fatalf("failed to create database directory: %v", err)
	}

	db, err := sql.Open("sqlite", connSpec+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}

	setupDatabase(db)
	return db
}

func setupDatabase(db *sql.DB) {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS files (
			absolute_file_path VARCHAR PRIMARY KEY,
			inserted_at TIMESTAMP NOT NULL,
			modified_at TIMESTAMP NOT NULL,
			accessed_at TIMESTAMP NOT NULL DEFAULT '1970-01-01 00:00:00',
			expiration TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS watches (
			absolute_file_path VARCHAR PRIMARY KEY,
			created_at TIMESTAMP NOT NULL,
			default_expiration INTEGER
		)`,
		`PRAGMA foreign_keys = ON`,
		`CREATE TABLE IF NOT EXISTS files_watches (
			file_id VARCHAR NOT NULL,
			watch_id VARCHAR NOT NULL,
			PRIMARY KEY (file_id, watch_id),
			FOREIGN KEY (file_id) REFERENCES files(absolute_file_path),
			FOREIGN KEY (watch_id) REFERENCES watches(absolute_file_path)
		)`,
		`CREATE TABLE IF NOT EXISTS settings (
			key VARCHAR PRIMARY KEY,
			value VARCHAR NOT NULL
		)`,
	}

	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			log.Fatalf("failed to execute setup query: %v\n%s", err, q)
		}
	}

	// Migration: add accessed_at column if missing
	db.Exec("ALTER TABLE files ADD COLUMN accessed_at TIMESTAMP NOT NULL DEFAULT '1970-01-01 00:00:00'")

	fmt.Println("Database initialized")
}
