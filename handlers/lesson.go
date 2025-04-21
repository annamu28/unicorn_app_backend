package handlers

import (
	"database/sql"
	"log"
	"net/http"
	"unicorn_app_backend/models"

	"github.com/gin-gonic/gin"
)

type LessonHandler struct {
	db *sql.DB
}

func NewLessonHandler(db *sql.DB) *LessonHandler {
	return &LessonHandler{db: db}
}

func (h *LessonHandler) checkPermission(userID int) (bool, error) {
	var hasPermission bool
	err := h.db.QueryRow(`
        SELECT EXISTS (
            SELECT 1 FROM user_roles ur
            JOIN roles r ON r.id = ur.role_id
            WHERE ur.user_id = $1 
            AND r.role IN ('Admin', 'Head Unicorn', 'Helper Unicorn')
        )
    `, userID).Scan(&hasPermission)

	return hasPermission, err
}

func (h *LessonHandler) CreateLesson(c *gin.Context) {
	userID := c.GetInt("userID")

	// Check if user has permission
	hasPermission, err := h.checkPermission(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify permissions"})
		return
	}

	if !hasPermission {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only Admins, Head Unicorns, and Helper Unicorns can manage lessons"})
		return
	}

	var req models.CreateLessonRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if course exists
	var courseExists bool
	err = h.db.QueryRow(`
        SELECT EXISTS (
            SELECT 1 FROM courses 
            WHERE id = $1
        )
    `, req.CourseID).Scan(&courseExists)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify course"})
		return
	}

	if !courseExists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Course not found"})
		return
	}

	// Create the lesson
	var lesson models.LessonResponse
	err = h.db.QueryRow(`
        INSERT INTO lessons (course_id, title, description, created_at)
        VALUES ($1, $2, $3, CURRENT_DATE)
        RETURNING id, course_id, title, description, created_at
    `, req.CourseID, req.Title, req.Description).Scan(
		&lesson.ID,
		&lesson.CourseID,
		&lesson.Title,
		&lesson.Description,
		&lesson.CreatedAt,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create lesson"})
		return
	}

	c.JSON(http.StatusCreated, lesson)
}

func (h *LessonHandler) GetLessons(c *gin.Context) {
	// Get optional course_id filter from query params
	courseID := c.Query("course_id")

	var rows *sql.Rows
	var err error

	if courseID != "" {
		// If course_id is provided, filter lessons by course
		rows, err = h.db.Query(`
            SELECT id, course_id, title, description, created_at
            FROM lessons
            WHERE course_id = $1
            ORDER BY created_at DESC
        `, courseID)
	} else {
		// If no course_id, get all lessons
		rows, err = h.db.Query(`
            SELECT id, course_id, title, description, created_at
            FROM lessons
            ORDER BY created_at DESC
        `)
	}

	if err != nil {
		log.Printf("Error fetching lessons: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch lessons"})
		return
	}
	defer rows.Close()

	var lessons []models.Lesson
	for rows.Next() {
		var lesson models.Lesson
		var createdAt sql.NullTime
		err := rows.Scan(
			&lesson.ID,
			&lesson.CourseID,
			&lesson.Title,
			&lesson.Description,
			&createdAt,
		)
		if err != nil {
			log.Printf("Error scanning lesson: %v", err)
			continue
		}
		if createdAt.Valid {
			lesson.CreatedAt = createdAt.Time.Format("2006-01-02")
		}
		lessons = append(lessons, lesson)
	}

	if lessons == nil {
		lessons = make([]models.Lesson, 0)
	}

	c.JSON(http.StatusOK, lessons)
}
