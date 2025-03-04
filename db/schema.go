package db

import (
	"database/sql"
	"fmt"
)

const Schema = `
-- Drop tables if they exist


-- Create users table
CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
	birthday DATE,
	first_name VARCHAR(50) NOT NULL,
	last_name VARCHAR(50) NOT NULL,
    username VARCHAR(50),
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Create roles table
CREATE TABLE IF NOT EXISTS roles (
	id SERIAL PRIMARY KEY,
	role VARCHAR(255) NOT NULL
	);

-- Create user_roles table
CREATE TABLE IF NOT EXISTS user_roles (
	id SERIAL PRIMARY KEY,
	user_id INTEGER NOT NULL,
    role_id INTEGER NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (role_id) REFERENCES roles(id) ON DELETE CASCADE,
    UNIQUE(user_id, role_id)
	);

-- Create squads table
CREATE TABLE IF NOT EXISTS squads (
	id SERIAL PRIMARY KEY,
	name VARCHAR(255) NOT NULL,
	);

-- Create user_squads table
CREATE TABLE IF NOT EXISTS user_squads (
	id SERIAL PRIMARY KEY,
	user_id INTEGER NOT NULL,
    squad_id INTEGER NOT NULL,
	status VARCHAR(50) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (squad_id) REFERENCES squads(id) ON DELETE CASCADE,
    UNIQUE(user_id, squad_id)
	);

--Create countries table
CREATE TABLE IF NOT EXISTS countries (
	id SERIAL PRIMARY KEY,
	name VARCHAR(255) NOT NULL,
	);

-- Create user_countries table
CREATE TABLE IF NOT EXISTS user_countries (
	id SERIAL PRIMARY KEY,
	user_id INTEGER NOT NULL,
    country_id INTEGER NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (country_id) REFERENCES squads(id) ON DELETE CASCADE,
    UNIQUE(user_id, country_id)
	);
`

// InitSchema initializes the database schema
func InitSchema(db *sql.DB) error {
	_, err := db.Exec(Schema)
	if err != nil {
		return fmt.Errorf("error initializing database schema: %w", err)
	}
	return nil
}
