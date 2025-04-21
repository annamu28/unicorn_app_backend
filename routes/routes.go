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
	chatboardHandler := handlers.NewChatboardHandler(db)
	postHandler := handlers.NewPostHandler(db)
	commentHandler := handlers.NewCommentHandler(db)
	courseHandler := handlers.NewCourseHandler(db)
	lessonHandler := handlers.NewLessonHandler(db)
	attendanceHandler := handlers.NewAttendanceHandler(db)
	testHandler := handlers.NewTestHandler(db)

	// Public routes
	r.POST("/register", authHandler.Register)
	r.POST("/login", authHandler.Login)
	r.POST("/refresh", authHandler.RefreshToken)

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
		protected.POST("/tests", testHandler.CreateTest)
		protected.GET("/tests", testHandler.GetTests)
		protected.GET("/tests/:id", testHandler.GetTestByID)
		protected.POST("/tests/attempt", testHandler.SubmitTestAttempt)
		protected.GET("/rewards", testHandler.GetUserRewards)
		protected.POST("/rewards", testHandler.CreateReward)
		protected.PUT("/rewards/:id", testHandler.UpdateReward)

		// Logout route
		protected.POST("/logout", authHandler.Logout)

		// User info route
		protected.GET("/userinfo", authHandler.GetUserInfo)

		// Verification route
		protected.POST("/verification", avatarHandler.VerifyUserSquad)
	}
}
