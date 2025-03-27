package handlers

import (
	"database/sql"
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
	userID := c.GetInt("userID")

	// Check if user has permission
	hasPermission, err := h.checkPermission(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify permissions"})
		return
	}

	if !hasPermission {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only Admins, Head Unicorns, and Helper Unicorns can view lessons"})
		return
	}

	courseID := c.Query("course_id")

	query := `
        SELECT 
            l.id,
            l.course_id,
            l.title,
            l.description,
            l.created_at,
            c.name as course_name,
            COUNT(a.id) as attendance_count
        FROM lessons l
        JOIN courses c ON c.id = l.course_id
        LEFT JOIN attendances a ON a.lesson_id = l.id
    `
	params := []interface{}{}

	if courseID != "" {
		query += " WHERE l.course_id = $1"
		params = append(params, courseID)
	}

	query += `
        GROUP BY 
            l.id, 
            l.course_id,
            l.title,
            l.description,
            l.created_at,
            c.name
        ORDER BY l.created_at DESC
    `

	rows, err := h.db.Query(query, params...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch lessons"})
		return
	}
	defer rows.Close()

	var lessons []struct {
		models.LessonResponse
		CourseName      string `json:"course_name"`
		AttendanceCount int    `json:"attendance_count"`
	}

	for rows.Next() {
		var lesson struct {
			models.LessonResponse
			CourseName      string `json:"course_name"`
			AttendanceCount int    `json:"attendance_count"`
		}
		err := rows.Scan(
			&lesson.ID,
			&lesson.CourseID,
			&lesson.Title,
			&lesson.Description,
			&lesson.CreatedAt,
			&lesson.CourseName,
			&lesson.AttendanceCount,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to scan lesson"})
			return
		}
		lessons = append(lessons, lesson)
	}

	c.JSON(http.StatusOK, lessons)
}
