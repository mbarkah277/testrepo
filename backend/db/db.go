// Package db provides the PostgreSQL connection pool for FamilySync.
package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "github.com/lib/pq"
)

// DB is the shared database connection pool.
var DB *sql.DB

// Connect initialises the PostgreSQL connection pool using the DB_DSN env var.
// It panics if the connection cannot be established.
func Connect() {
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		log.Fatal("DB_DSN environment variable is not set")
	}

	var err error
	DB, err = sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("db: failed to open connection: %v", err)
	}

	// Verify the connection is alive.
	if err = DB.Ping(); err != nil {
		log.Fatalf("db: failed to ping database: %v", err)
	}

	// Connection pool settings suitable for Armbian / 2 GB RAM.
	DB.SetMaxOpenConns(25)
	DB.SetMaxIdleConns(5)

	fmt.Println("✅  PostgreSQL connected")
}
