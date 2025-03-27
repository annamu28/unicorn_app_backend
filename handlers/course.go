package handlers

import (
	"database/sql"
	"net/http"
	"time"
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
		c.JSON(http.StatusForbidden, gin.H{"error": "Only admins can view courses"})
		return
	}

	// Get all courses with their lessons
	rows, err := h.db.Query(`
        SELECT 
            c.id,
            c.name,
            c.created_at,
            l.id,
            l.title,
            l.description,
            l.created_at,
            COUNT(a.id) as attendance_count
        FROM courses c
        LEFT JOIN lessons l ON l.course_id = c.id
        LEFT JOIN attendances a ON a.lesson_id = l.id
        GROUP BY 
            c.id, 
            c.name, 
            c.created_at,
            l.id,
            l.title,
            l.description,
            l.created_at
        ORDER BY 
            c.created_at DESC,
            l.created_at ASC
    `)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch courses"})
		return
	}
	defer rows.Close()

	// Map to store courses by ID
	coursesMap := make(map[int]*models.CourseWithLessonsResponse)

	for rows.Next() {
		var courseID int
		var courseName string
		var courseCreatedAt time.Time
		var lessonID sql.NullInt64
		var lessonTitle, lessonDescription sql.NullString
		var lessonCreatedAt sql.NullTime
		var attendanceCount int

		err := rows.Scan(
			&courseID,
			&courseName,
			&courseCreatedAt,
			&lessonID,
			&lessonTitle,
			&lessonDescription,
			&lessonCreatedAt,
			&attendanceCount,
		)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to scan course data"})
			return
		}

		// Get or create course in map
		course, exists := coursesMap[courseID]
		if !exists {
			course = &models.CourseWithLessonsResponse{
				ID:        courseID,
				Name:      courseName,
				CreatedAt: courseCreatedAt,
				Lessons:   []models.LessonResponse{},
			}
			coursesMap[courseID] = course
		}

		// Add lesson if it exists
		if lessonID.Valid {
			lesson := models.LessonResponse{
				ID:          int(lessonID.Int64),
				CourseID:    courseID,
				Title:       lessonTitle.String,
				Description: lessonDescription.String,
				CreatedAt:   lessonCreatedAt.Time,
			}
			course.Lessons = append(course.Lessons, lesson)
		}
	}

	// Convert map to slice
	var courses []models.CourseWithLessonsResponse
	for _, course := range coursesMap {
		courses = append(courses, *course)
	}

	c.JSON(http.StatusOK, courses)
}
