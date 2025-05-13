package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	"unicorn_app_backend/db"
	"unicorn_app_backend/routes"

	_ "github.com/lib/pq"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	// Add godotenv at the start of main()
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found") // Non-fatal in production
	}

	// Get port from environment variable
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Database configuration for Docker
	dbHost := os.Getenv("DB_HOST")
	if dbHost == "" {
		dbHost = "db" // Default Docker service name
	}
	dbPort := os.Getenv("DB_PORT")
	if dbPort == "" {
		dbPort = "5432" // Default PostgreSQL port
	}
	dbUser := os.Getenv("DB_USER")
	if dbUser == "" {
		dbUser = "postgres" // Default user
	}
	dbPassword := os.Getenv("DB_PASSWORD")
	if dbPassword == "" {
		log.Fatal("DB_PASSWORD environment variable is required")
	}
	dbName := os.Getenv("DB_NAME")
	if dbName == "" {
		dbName = "postgres" // Default database name
	}

	// Construct database connection string for Docker
	dbURL := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		dbHost, dbPort, dbUser, dbPassword, dbName)

	// Get JWT secret from environment variable
	jwtSecret := []byte(os.Getenv("JWT_SECRET"))
	if len(jwtSecret) == 0 {
		log.Fatal("JWT_SECRET environment variable is required")
	}

	// Connect to database
	database, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("Error connecting to database: %v", err)
	}
	defer database.Close()

	// Test database connection
	if err := database.Ping(); err != nil {
		log.Fatalf("Error connecting to the database: %v", err)
	}

	// Initialize database schema
	if err := db.InitSchema(database); err != nil {
		log.Fatalf("Error initializing database schema: %v", err)
	}

	// Seed initial data
	//if err := db.SeedData(database); err != nil {
	//	log.Printf("Warning: Error seeding initial data: %v", err)
	//}


	// Initialize router
	r := gin.Default()

	// Setup CORS - Simplified for mobile app
	config := cors.DefaultConfig()
	config.AllowAllOrigins = true // Allow all origins for mobile app
	config.AllowHeaders = []string{
		"Origin",
		"Content-Length",
		"Content-Type",
		"Authorization",
	}
	config.AllowMethods = []string{
		"GET",
		"POST",
		"PUT",
		"DELETE",
		"PATCH",
	}
	r.Use(cors.New(config))

	// Setup routes
	routes.SetupRoutes(r, database, jwtSecret)

	// Run server
	srv := &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown:", err)
	}
}
