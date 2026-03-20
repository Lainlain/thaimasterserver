//go:build ignore
// +build ignore

// verify-postgres.go — standalone tool to verify all tables migrated correctly
// Usage: go run verify-postgres.go
// Set env: POSTGRES_DSN="postgres://user:pass@localhost:5432/thaimaster2d?sslmode=disable"

package main

import (
	"database/sql"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	_ "github.com/lib/pq"
)

type TableCheck struct {
	Name        string
	SampleQuery string // optional: a quick sanity query
}

var tables = []TableCheck{
	{Name: "twodhistory", SampleQuery: "SELECT date, result430 FROM twodhistory ORDER BY date DESC LIMIT 1"},
	{Name: "gifts", SampleQuery: "SELECT id, name FROM gifts LIMIT 1"},
	{Name: "sliders", SampleQuery: "SELECT id, image_url FROM sliders LIMIT 1"},
	{Name: "threed", SampleQuery: "SELECT id, date FROM threed ORDER BY date DESC LIMIT 1"},
	{Name: "paper_types", SampleQuery: "SELECT id, name FROM paper_types LIMIT 1"},
	{Name: "paper_images", SampleQuery: "SELECT id, type_id FROM paper_images LIMIT 1"},
	{Name: "numerology_cache", SampleQuery: "SELECT id FROM numerology_cache LIMIT 1"},
	{Name: "chat_users", SampleQuery: "SELECT id, display_name FROM chat_users LIMIT 1"},
	{Name: "chat_device_bans", SampleQuery: "SELECT id FROM chat_device_bans LIMIT 1"},
	{Name: "chat_messages", SampleQuery: "SELECT id, message FROM chat_messages ORDER BY id DESC LIMIT 1"},
	{Name: "chat_reports", SampleQuery: "SELECT id FROM chat_reports LIMIT 1"},
}

func main() {
	dsn := os.Getenv("POSTGRES_DSN")
	if dsn == "" {
		// Default — edit this if needed
		dsn = "postgres://postgres:password@localhost:5432/thaimaster2d?sslmode=disable"
	}

	fmt.Println("🔌 Connecting to PostgreSQL...")
	fmt.Printf("   DSN: %s\n\n", maskPassword(dsn))

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		fatal("Failed to open DB: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		fatal("Cannot reach PostgreSQL: %v\n\nCheck:\n  - Is PostgreSQL running?\n  - Is POSTGRES_DSN correct?", err)
	}
	fmt.Println("✅ Connected!\n")

	// ── Row count check ──────────────────────────────────────────────────
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "TABLE\tROWS\tSTATUS\tSAMPLE")
	fmt.Fprintln(w, "─────────────────\t──────\t───────\t──────────────────────────────────")

	allOk := true
	totalRows := 0

	for _, t := range tables {
		// Count rows
		var count int
		err := db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", t.Name)).Scan(&count)
		if err != nil {
			fmt.Fprintf(w, "%s\t ERROR\t❌\t%v\n", t.Name, err)
			allOk = false
			continue
		}
		totalRows += count

		status := "✅ OK"
		if count == 0 {
			status = "⚠️  EMPTY"
		}

		// Sample row
		sample := ""
		if count > 0 && t.SampleQuery != "" {
			sample = querySample(db, t.SampleQuery)
		}

		fmt.Fprintf(w, "%s\t%d\t%s\t%s\n", t.Name, count, status, sample)
	}
	w.Flush()

	// ── Summary ──────────────────────────────────────────────────────────
	fmt.Printf("\n─────────────────────────────────────────\n")
	fmt.Printf("Total rows across all tables: %d\n", totalRows)

	if allOk {
		fmt.Println("\n🎉 All tables verified successfully!")
	} else {
		fmt.Println("\n⚠️  Some tables had errors — check above.")
		os.Exit(1)
	}
}

// querySample runs a single-row query and returns columns joined as "col1=val1, col2=val2"
func querySample(db *sql.DB, query string) string {
	rows, err := db.Query(query)
	if err != nil {
		return fmt.Sprintf("query err: %v", err)
	}
	defer rows.Close()

	cols, _ := rows.Columns()
	if !rows.Next() {
		return "(no rows)"
	}

	vals := make([]interface{}, len(cols))
	ptrs := make([]interface{}, len(cols))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	rows.Scan(ptrs...)

	parts := make([]string, 0, len(cols))
	for i, col := range cols {
		parts = append(parts, fmt.Sprintf("%s=%v", col, vals[i]))
	}
	result := strings.Join(parts, ", ")
	if len(result) > 60 {
		result = result[:57] + "..."
	}
	return result
}

func maskPassword(dsn string) string {
	// e.g. postgres://user:PASSWORD@host → postgres://user:***@host
	if i := strings.Index(dsn, "://"); i >= 0 {
		rest := dsn[i+3:]
		if j := strings.Index(rest, "@"); j >= 0 {
			creds := rest[:j]
			if k := strings.Index(creds, ":"); k >= 0 {
				return dsn[:i+3] + creds[:k+1] + "***" + "@" + rest[j+1:]
			}
		}
	}
	return dsn
}

func fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "❌ "+format+"\n", args...)
	os.Exit(1)
}
