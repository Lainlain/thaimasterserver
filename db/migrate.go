package db

import (
	"database/sql"
	"fmt"
	"log"
	"net/url"
	"strings"

	_ "github.com/lib/pq"
)

// Connect connects to PostgreSQL, auto-creates the database if it doesn't exist,
// runs all table migrations, and returns the ready *sql.DB connection.
func Connect(dsn string) (*sql.DB, error) {
	dbName, err := extractDBName(dsn)
	if err != nil {
		return nil, fmt.Errorf("invalid DSN: %w", err)
	}

	// Step 1: Connect to default 'postgres' DB to create our DB if needed
	adminDSN := replaceDBName(dsn, "postgres")
	adminDB, err := sql.Open("postgres", adminDSN)
	if err != nil {
		return nil, fmt.Errorf("failed to open admin connection: %w", err)
	}
	if err := adminDB.Ping(); err != nil {
		adminDB.Close()
		return nil, fmt.Errorf("failed to ping postgres: %w", err)
	}

	var exists bool
	row := adminDB.QueryRow("SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)", dbName)
	if err := row.Scan(&exists); err != nil {
		adminDB.Close()
		return nil, fmt.Errorf("failed to check database existence: %w", err)
	}
	if !exists {
		log.Printf("📦 Database '%s' not found — creating...", dbName)
		_, err = adminDB.Exec(fmt.Sprintf(`CREATE DATABASE "%s"`, dbName))
		if err != nil {
			adminDB.Close()
			return nil, fmt.Errorf("failed to create database: %w", err)
		}
		log.Printf("✅ Database '%s' created", dbName)
	} else {
		log.Printf("✅ Database '%s' already exists", dbName)
	}
	adminDB.Close()

	// Step 2: Connect to our actual database
	database, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	if err := database.Ping(); err != nil {
		database.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Step 3: Run all table migrations
	if err := runMigrations(database); err != nil {
		database.Close()
		return nil, fmt.Errorf("migration failed: %w", err)
	}

	log.Println("✅ All migrations applied successfully")
	return database, nil
}

// runMigrations creates all tables if they don't exist.
// Safe to run on every startup — uses CREATE TABLE IF NOT EXISTS everywhere.
func runMigrations(db *sql.DB) error {
	type migration struct {
		name string
		sql  string
	}

	migrations := []migration{
		{
			name: "twodhistory",
			sql: `
CREATE TABLE IF NOT EXISTS twodhistory (
id          SERIAL PRIMARY KEY,
date        TEXT NOT NULL UNIQUE,
set1200     TEXT,
value1200   TEXT,
result1200  TEXT,
set430      TEXT,
value430    TEXT,
result430   TEXT,
modern930   TEXT,
internet930 TEXT,
modern200   TEXT,
internet200 TEXT,
created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_twodhistory_date ON twodhistory(date DESC);
`,
		},
		{
			name: "gifts",
			sql: `
CREATE TABLE IF NOT EXISTS gifts (
id          SERIAL PRIMARY KEY,
name        TEXT NOT NULL,
image_link  TEXT NOT NULL,
type        TEXT NOT NULL CHECK (type IN ('Daily', 'Weekly')),
description TEXT,
points      INTEGER DEFAULT 0,
stock       INTEGER DEFAULT 0,
is_active   INTEGER DEFAULT 1,
created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_gift_type   ON gifts(type);
CREATE INDEX IF NOT EXISTS idx_gift_active ON gifts(is_active);
`,
		},
		{
			name: "sliders",
			sql: `
CREATE TABLE IF NOT EXISTS sliders (
id           SERIAL PRIMARY KEY,
image_link   TEXT NOT NULL,
forward_link TEXT,
title        TEXT,
order_num    INTEGER DEFAULT 0,
is_active    INTEGER DEFAULT 1,
created_at   TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_slider_active ON sliders(is_active);
CREATE INDEX IF NOT EXISTS idx_slider_order  ON sliders(order_num);
`,
		},
		{
			name: "threed",
			sql: `
CREATE TABLE IF NOT EXISTS threed (
id         SERIAL PRIMARY KEY,
date       DATE NOT NULL UNIQUE,
result     TEXT NOT NULL,
created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_threed_date ON threed(date DESC);
`,
		},
		{
			name: "paper_types",
			sql: `
CREATE TABLE IF NOT EXISTS paper_types (
id            SERIAL PRIMARY KEY,
name          TEXT NOT NULL UNIQUE,
display_order INTEGER NOT NULL DEFAULT 0,
is_active     INTEGER NOT NULL DEFAULT 1,
created_at    TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
updated_at    TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_paper_types_display_order ON paper_types(display_order);
`,
		},
		{
			name: "paper_images",
			sql: `
CREATE TABLE IF NOT EXISTS paper_images (
id            SERIAL PRIMARY KEY,
type_id       INTEGER NOT NULL REFERENCES paper_types(id) ON DELETE CASCADE,
image_url     TEXT NOT NULL,
display_order INTEGER NOT NULL DEFAULT 0,
is_active     INTEGER NOT NULL DEFAULT 1,
created_at    TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
updated_at    TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_paper_images_type_id      ON paper_images(type_id);
CREATE INDEX IF NOT EXISTS idx_paper_images_display_order ON paper_images(display_order);
`,
		},
		{
			name: "numerology_cache",
			sql: `
CREATE TABLE IF NOT EXISTS numerology_cache (
birthdate   TEXT NOT NULL,
result_json TEXT NOT NULL,
cached_date TEXT NOT NULL,
updated_at  TEXT NOT NULL,
PRIMARY KEY (birthdate, cached_date)
);
`,
		},
		// ── Chat tables ──────────────────────────────────────────────────
		{
			name: "chat_users",
			sql: `
				CREATE TABLE IF NOT EXISTS chat_users (
					id           SERIAL PRIMARY KEY,
					google_id    TEXT NOT NULL UNIQUE,
					display_name TEXT NOT NULL,
					avatar_url   TEXT NOT NULL DEFAULT '',
					role         TEXT NOT NULL DEFAULT 'user',
					is_banned    BOOLEAN NOT NULL DEFAULT FALSE,
					ban_reason   TEXT NOT NULL DEFAULT '',
					created_at   TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
					updated_at   TIMESTAMP DEFAULT CURRENT_TIMESTAMP
				);
				CREATE INDEX IF NOT EXISTS idx_chat_users_google_id ON chat_users(google_id);
			`,
		},
		{
			name: "chat_device_bans",
			sql: `
				CREATE TABLE IF NOT EXISTS chat_device_bans (
					id         SERIAL PRIMARY KEY,
					device_id  TEXT NOT NULL UNIQUE,
					ban_reason TEXT NOT NULL DEFAULT '',
					banned_by  INTEGER REFERENCES chat_users(id),
					created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
				);
				CREATE INDEX IF NOT EXISTS idx_chat_device_bans_device_id ON chat_device_bans(device_id);
			`,
		},
		{
			name: "chat_messages",
			sql: `
				CREATE TABLE IF NOT EXISTS chat_messages (
					id           SERIAL PRIMARY KEY,
					user_id      INTEGER NOT NULL REFERENCES chat_users(id) ON DELETE CASCADE,
					display_name TEXT NOT NULL,
					avatar_url   TEXT NOT NULL DEFAULT '',
					message      TEXT NOT NULL,
					created_at   TIMESTAMP DEFAULT CURRENT_TIMESTAMP
				);
				CREATE INDEX IF NOT EXISTS idx_chat_messages_created_at ON chat_messages(created_at DESC);
				CREATE INDEX IF NOT EXISTS idx_chat_messages_user_id    ON chat_messages(user_id);
			`,
		},
		{
			name: "chat_reports",
			sql: `
				CREATE TABLE IF NOT EXISTS chat_reports (
					id                 SERIAL PRIMARY KEY,
					reported_user_id   INTEGER NOT NULL REFERENCES chat_users(id) ON DELETE CASCADE,
					message_id         INTEGER REFERENCES chat_messages(id) ON DELETE SET NULL,
					reporter_device_id TEXT NOT NULL DEFAULT '',
					reason             TEXT NOT NULL DEFAULT '',
					is_reviewed        BOOLEAN NOT NULL DEFAULT FALSE,
					created_at         TIMESTAMP DEFAULT CURRENT_TIMESTAMP
				);
				CREATE INDEX IF NOT EXISTS idx_chat_reports_reported_user_id ON chat_reports(reported_user_id);
				CREATE INDEX IF NOT EXISTS idx_chat_reports_created_at       ON chat_reports(created_at DESC);
			`,
		},
	}

	for _, m := range migrations {
		if _, err := db.Exec(m.sql); err != nil {
			return fmt.Errorf("failed to apply migration '%s': %w", m.name, err)
		}
		log.Printf("  ✅ Migration applied: %s", m.name)
	}

	seedPaperTypes(db)
	return nil
}

// seedPaperTypes inserts default paper categories if the table is empty.
func seedPaperTypes(db *sql.DB) {
	var count int
	db.QueryRow("SELECT COUNT(*) FROM paper_types").Scan(&count)
	if count == 0 {
		log.Println("🌱 Seeding default paper types...")
		db.Exec(`
INSERT INTO paper_types (name, display_order) VALUES
('Myanmar News', 1),
('Thailand News', 2),
('International', 3)
`)
	}
}

// extractDBName parses the database name from a postgres DSN URL.
func extractDBName(dsn string) (string, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return "", err
	}
	name := strings.TrimPrefix(u.Path, "/")
	if name == "" {
		return "", fmt.Errorf("no database name in DSN")
	}
	return name, nil
}

// replaceDBName returns the DSN with the target database name replaced.
func replaceDBName(dsn, newDB string) string {
	u, err := url.Parse(dsn)
	if err != nil {
		return dsn
	}
	u.Path = "/" + newDB
	return u.String()
}
