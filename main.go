package main

import (
	"log"
	"unicorn_app_backend/config"
	"unicorn_app_backend/db"
	"unicorn_app_backend/routes"

	"github.com/gin-gonic/gin"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatal("Failed to load configuration:", err)
	}

	// Add debug logging
	log.Printf("Database config: host=%s port=%d user=%s dbname=%s",
		cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBName)

	// Set Gin mode based on environment
	if cfg.Environment != "development" {
		gin.SetMode(gin.ReleaseMode)
	}

	// Initialize database
	dbConfig := db.Config{
		Host:     cfg.DBHost,
		Port:     cfg.DBPort,
		User:     cfg.DBUser,
		Password: cfg.DBPassword,
		DBName:   cfg.DBName,
	}

	database, err := db.Initialize(dbConfig)
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()

	// Initialize schema
	if err := db.InitSchema(database); err != nil {
		log.Fatal(err)
	}

	// Initialize Gin router
	r := gin.Default()

	// Setup routes
	routes.SetupRoutes(r, database, []byte(cfg.JWTSecret))

	// Start server
	log.Printf("Server starting on port %s...\n", cfg.ServerPort)
	if err := r.Run(":" + cfg.ServerPort); err != nil {
		log.Fatal(err)
	}
}
