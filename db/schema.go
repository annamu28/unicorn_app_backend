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
    created_at DATE DEFAULT CURRENT_DATE
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
    created_at DATE DEFAULT CURRENT_DATE,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (role_id) REFERENCES roles(id) ON DELETE CASCADE,
    UNIQUE(user_id, role_id)
);

-- Create squads table
CREATE TABLE IF NOT EXISTS squads (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL
);

-- Create user_squads table
CREATE TABLE IF NOT EXISTS user_squads (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL,
    squad_id INTEGER NOT NULL,
    status VARCHAR(50) NOT NULL,
    created_at DATE DEFAULT CURRENT_DATE,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (squad_id) REFERENCES squads(id) ON DELETE CASCADE,
    UNIQUE(user_id, squad_id)
);

--Create countries table
CREATE TABLE IF NOT EXISTS countries (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL
);

-- Create user_countries table
CREATE TABLE IF NOT EXISTS user_countries (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL,
    country_id INTEGER NOT NULL,
    created_at DATE DEFAULT CURRENT_DATE,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (country_id) REFERENCES countries(id) ON DELETE CASCADE,
    UNIQUE(user_id, country_id)
);

-- Create refresh_tokens table
CREATE TABLE IF NOT EXISTS refresh_tokens (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL,
    token VARCHAR(255) UNIQUE NOT NULL,
    expires_at DATE,
    created_at DATE DEFAULT CURRENT_DATE,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- Create chatboards table
CREATE TABLE IF NOT EXISTS chatboards (
    id SERIAL PRIMARY KEY,
    title VARCHAR(255) NOT NULL,
    description VARCHAR(255) NOT NULL,
    created_at DATE DEFAULT CURRENT_DATE
);
    
-- Create chatboard_squads table
CREATE TABLE IF NOT EXISTS chatboard_squads (
    id SERIAL PRIMARY KEY,
    chatboard_id INTEGER NOT NULL,
    squad_id INTEGER NOT NULL,
    created_at DATE DEFAULT CURRENT_DATE,
    FOREIGN KEY (chatboard_id) REFERENCES chatboards(id) ON DELETE CASCADE,
    FOREIGN KEY (squad_id) REFERENCES squads(id) ON DELETE CASCADE,
    UNIQUE(chatboard_id, squad_id)
);

-- Create chatboard_roles table
CREATE TABLE IF NOT EXISTS chatboard_roles (
    id SERIAL PRIMARY KEY,
    chatboard_id INTEGER NOT NULL,
    role_id INTEGER NOT NULL,
    created_at DATE DEFAULT CURRENT_DATE,
    FOREIGN KEY (chatboard_id) REFERENCES chatboards(id) ON DELETE CASCADE,
    FOREIGN KEY (role_id) REFERENCES roles(id) ON DELETE CASCADE,
    UNIQUE(chatboard_id, role_id)
);

-- Create chatboard_countries table
CREATE TABLE IF NOT EXISTS chatboard_countries (
    id SERIAL PRIMARY KEY,
    chatboard_id INTEGER NOT NULL,
    country_id INTEGER NOT NULL,
    created_at DATE DEFAULT CURRENT_DATE,
    FOREIGN KEY (chatboard_id) REFERENCES chatboards(id) ON DELETE CASCADE,
    FOREIGN KEY (country_id) REFERENCES countries(id) ON DELETE CASCADE,
    UNIQUE(chatboard_id, country_id)
);

-- Create squad_roles table to link roles to specific squads
CREATE TABLE IF NOT EXISTS squad_roles (
    id SERIAL PRIMARY KEY,
    squad_id INTEGER NOT NULL,
    role_id INTEGER NOT NULL,
    created_at DATE DEFAULT CURRENT_DATE,
    FOREIGN KEY (squad_id) REFERENCES squads(id) ON DELETE CASCADE,
    FOREIGN KEY (role_id) REFERENCES roles(id) ON DELETE CASCADE,
    UNIQUE(squad_id, role_id)
);

-- Create posts table 
CREATE TABLE IF NOT EXISTS posts (
    id SERIAL PRIMARY KEY,
    chatboard_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    title VARCHAR(255) NOT NULL,
    content TEXT NOT NULL,
    pinned BOOLEAN DEFAULT FALSE,
    created_at DATE DEFAULT CURRENT_DATE,
    FOREIGN KEY (chatboard_id) REFERENCES chatboards(id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- Create comments table 
CREATE TABLE IF NOT EXISTS comments (
    id SERIAL PRIMARY KEY,
    post_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    comment TEXT NOT NULL,
    created_at DATE DEFAULT CURRENT_DATE,
    FOREIGN KEY (post_id) REFERENCES posts(id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- Create courses table 
CREATE TABLE IF NOT EXISTS courses (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    created_at DATE DEFAULT CURRENT_DATE
);

-- Create lessons table 
CREATE TABLE IF NOT EXISTS lessons (
    id SERIAL PRIMARY KEY,
    course_id INTEGER NOT NULL,
    title VARCHAR(255) NOT NULL,
    description TEXT NOT NULL,
    created_at DATE DEFAULT CURRENT_DATE,
    FOREIGN KEY (course_id) REFERENCES courses(id) ON DELETE CASCADE
);

-- Create attendances table 
CREATE TABLE IF NOT EXISTS attendances (
    id SERIAL PRIMARY KEY,
    lesson_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    status VARCHAR(50) NOT NULL,
    created_at DATE DEFAULT CURRENT_DATE,
    FOREIGN KEY (course_id) REFERENCES courses(id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
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
