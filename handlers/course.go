package handlers

import (
	"database/sql"
	"log"
	"net/http"
	"unicorn_app_backend/models"

	"github.com/gin-gonic/gin"
)

type CourseHandler struct {
	db *sql.DB
}

func NewCourseHandler(db *sql.DB) *CourseHandler {
	return &CourseHandler{db: db}
}

func (h *CourseHandler) CreateCourse(c *gin.Context) {
	userID := c.GetInt("userID")

	// Check if user is an admin
	var isAdmin bool
	err := h.db.QueryRow(`
        SELECT EXISTS (
            SELECT 1 FROM user_roles ur
            JOIN roles r ON r.id = ur.role_id
            WHERE ur.user_id = $1 AND r.role = 'Admin'
        )
    `, userID).Scan(&isAdmin)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify admin status"})
		return
	}

	if !isAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only admins can create courses"})
		return
	}

	var req models.CreateCourseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Create the course
	var course models.CourseResponse
	err = h.db.QueryRow(`
        INSERT INTO courses (name, created_at)
        VALUES ($1, CURRENT_DATE)
        RETURNING id, name, created_at
    `, req.Name).Scan(&course.ID, &course.Name, &course.CreatedAt)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create course"})
		return
	}

	c.JSON(http.StatusCreated, course)
}

func (h *CourseHandler) GetCourses(c *gin.Context) {
	// No permission check needed - all authenticated users can access
	rows, err := h.db.Query(`
        SELECT id, name, created_at
        FROM courses
        ORDER BY created_at DESC
    `)
	if err != nil {
		log.Printf("Error fetching courses: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch courses"})
		return
	}
	defer rows.Close()

	var courses []models.Course
	for rows.Next() {
		var course models.Course
		var createdAt sql.NullTime
		err := rows.Scan(&course.ID, &course.Name, &createdAt)
		if err != nil {
			log.Printf("Error scanning course: %v", err)
			continue
		}
		if createdAt.Valid {
			course.CreatedAt = createdAt.Time.Format("2006-01-02")
		}
		courses = append(courses, course)
	}

	if courses == nil {
		courses = make([]models.Course, 0)
	}

	c.JSON(http.StatusOK, courses)
}
