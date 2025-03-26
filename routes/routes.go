package routes

import (
	"database/sql"

	"unicorn_app_backend/handlers"

	"github.com/gin-gonic/gin"
)

// SetupRoutes configures all the routes for the application
func SetupRoutes(r *gin.Engine, db *sql.DB, jwtSecret []byte) {
	// Initialize handlers
	authHandler := handlers.NewAuthHandler(db, jwtSecret)
	countryHandler := handlers.NewCountryHandler(db)
	roleHandler := handlers.NewRoleHandler(db)
	squadHandler := handlers.NewSquadHandler(db)
	avatarHandler := handlers.NewAvatarHandler(db)

	// Public routes
	r.POST("/register", authHandler.Register)
	r.POST("/login", authHandler.Login)

	// Protected routes
	protected := r.Group("/")
	protected.Use(authHandler.AuthMiddleware()) // You'll need to implement this middleware
	{
		//Avatar route
		protected.POST("/avatar", avatarHandler.CreateUserAvatar)

		//Country routes
		protected.POST("/countries", countryHandler.CreateCountry)
		protected.GET("/countries", countryHandler.GetCountries)

		//Role routes
		protected.POST("/roles", roleHandler.CreateRole)
		protected.GET("/roles", roleHandler.GetRoles)

		// Squad routes
		protected.POST("/squads", squadHandler.CreateSquad)
		protected.GET("/squads", squadHandler.GetSquads)
	}
}
