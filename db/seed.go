package db

import (
	"database/sql"
	"fmt"
)

// SeedData populates the database with initial data
func SeedData(db *sql.DB) error {
	// Start a transaction
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("error starting transaction: %w", err)
	}

	// Seed roles
	roles := []string{"Admin", "Head Unicorn", "Helper Unicorn", "Unicorn"}
	for _, role := range roles {
		_, err = tx.Exec("INSERT INTO roles (role) VALUES ($1) ON CONFLICT DO NOTHING", role)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("error seeding roles: %w", err)
		}
	}

	// Seed countries
	countries := []string{"Estonia"}
	for _, country := range countries {
		_, err = tx.Exec("INSERT INTO countries (name) VALUES ($1) ON CONFLICT DO NOTHING", country)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("error seeding countries: %w", err)
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("error committing transaction: %w", err)
	}

	return nil
}
