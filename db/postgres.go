package db

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/lib/pq"
)

type Config struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
}

var DB *sql.DB

// Initialize creates a new database connection and returns it
func Initialize(cfg Config) (*sql.DB, error) {
	// Use connection URL format instead
	psqlInfo := fmt.Sprintf("postgresql://%s:%s@%s:%d/%s?sslmode=disable",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.DBName)

	log.Printf("Attempting to connect with: %s", psqlInfo)

	db, err := sql.Open("postgres", psqlInfo)
	if err != nil {
		return nil, fmt.Errorf("error connecting to the database: %w", err)
	}

	if err = db.Ping(); err != nil {
		return nil, fmt.Errorf("error pinging database: %w", err)
	}

	DB = db
	return db, nil
}

// GetDB returns the database instance
func GetDB() *sql.DB {
	return DB
}
