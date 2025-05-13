package routes

import (
	"database/sql"

	"unicorn_app_backend/handlers"
	"unicorn_app_backend/middleware"

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
	chatboardHandler := handlers.NewChatboardHandler(db)
	postHandler := handlers.NewPostHandler(db)
	commentHandler := handlers.NewCommentHandler(db)
	courseHandler := handlers.NewCourseHandler(db)
	lessonHandler := handlers.NewLessonHandler(db)
	attendanceHandler := handlers.NewAttendanceHandler(db)
	testHandler := handlers.NewTestHandler(db)
	healthHandler := handlers.NewHealthHandler(db)
	userHandler := handlers.NewUserHandler(db)

	// Public routes
	r.GET("/health", healthHandler.HealthCheck)

	// Auth routes
	r.POST("/register", authHandler.Register)
	r.POST("/login", authHandler.Login)
	r.POST("/refresh", authHandler.RefreshToken)
	r.POST("/logout", authHandler.Logout)

	// Protected routes
	protected := r.Group("/")
	protected.Use(middleware.AuthMiddleware(db, jwtSecret)) // Pass the required parameters
	{
		//Avatar route
		protected.POST("/avatar", avatarHandler.CreateUserAvatar)

		//Country routes
		protected.POST("/countries", countryHandler.CreateCountry)
		protected.GET("/countries", countryHandler.GetCountries)

		//Role routes
		protected.POST("/roles", roleHandler.CreateRole)
		protected.GET("/roles", roleHandler.GetRoles)
		protected.POST("/roles/assign", roleHandler.AssignGlobalRole)

		// Squad routes
		protected.POST("/squads", squadHandler.CreateSquad)
		protected.GET("/squads", squadHandler.GetSquads)

		// Chatboard routes
		protected.POST("/chatboards", chatboardHandler.CreateChatboard)
		protected.GET("/chatboards", chatboardHandler.GetChatboards)
		protected.GET("/chatboards/:id/pending-users", chatboardHandler.GetPendingUsers)

		// Post routes
		protected.POST("/posts", postHandler.CreatePost)
		protected.GET("/posts", postHandler.GetPosts)
		protected.POST("/posts/:id/toggle-pin", postHandler.TogglePin)

		// Comment routes
		protected.POST("/comments", commentHandler.CreateComment)
		protected.GET("/comments", commentHandler.GetComments)

		// Course routes
		protected.POST("/courses", courseHandler.CreateCourse)
		protected.GET("/courses", courseHandler.GetCourses)

		// Lesson routes
		protected.POST("/lessons", lessonHandler.CreateLesson)
		protected.GET("/lessons", lessonHandler.GetLessons)

		// Attendance routes
		protected.POST("/attendances", attendanceHandler.CreateAttendance)
		protected.GET("/attendances", attendanceHandler.GetAttendances)
		protected.DELETE("/attendances/:id", attendanceHandler.DeleteAttendance)

		// Test routes
		testRoutes := protected.Group("/tests")
		{
			testRoutes.GET("", testHandler.GetTests)
			testRoutes.GET("/:id", testHandler.GetTestByID)
			testRoutes.POST("", testHandler.CreateTest)
			testRoutes.POST("/attempt", testHandler.SubmitTestAttempt)
			testRoutes.GET("/rewards", testHandler.GetUserRewards)
			testRoutes.POST("/rewards", testHandler.CreateReward)
			testRoutes.PUT("/rewards/:id", testHandler.UpdateReward)
			testRoutes.GET("/rewards-catalog", testHandler.GetRewardsCatalog)
			testRoutes.POST("/rewards-catalog", testHandler.CreateRewardCatalog)
			testRoutes.PUT("/rewards-catalog/:id", testHandler.UpdateRewardCatalog)
			testRoutes.DELETE("/rewards-catalog/:id", testHandler.DeleteRewardCatalog)

			// Chatboard test routes
			testRoutes.POST("/chatboard/activate", testHandler.ActivateTestInChatboard)
			testRoutes.POST("/chatboard/deactivate", testHandler.DeactivateTestInChatboard)
			testRoutes.GET("/chatboard/:chatboard_id", testHandler.GetChatboardTests)
		}

		// User info route
		protected.GET("/userinfo", userHandler.GetUserInfo)

		// Verification route
		protected.POST("/verification", avatarHandler.VerifyUserSquad)
	}
}
