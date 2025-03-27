package handlers

import (
	"database/sql"
	"net/http"
	"unicorn_app_backend/models"

	"github.com/gin-gonic/gin"
)

type AttendanceHandler struct {
	db *sql.DB
}

func NewAttendanceHandler(db *sql.DB) *AttendanceHandler {
	return &AttendanceHandler{db: db}
}

func (h *AttendanceHandler) checkPermission(userID int) (bool, error) {
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

func (h *AttendanceHandler) CreateAttendance(c *gin.Context) {
	userID := c.GetInt("userID")

	// Check if user has permission
	hasPermission, err := h.checkPermission(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify permissions"})
		return
	}

	if !hasPermission {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only Admins, Head Unicorns, and Helper Unicorns can manage attendance"})
		return
	}

	var req models.CreateAttendanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if lesson exists
	var lessonExists bool
	err = h.db.QueryRow(`
        SELECT EXISTS (
            SELECT 1 FROM lessons 
            WHERE id = $1
        )
    `, req.LessonID).Scan(&lessonExists)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify lesson"})
		return
	}

	if !lessonExists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Lesson not found"})
		return
	}

	// Check if user exists
	var userExists bool
	err = h.db.QueryRow(`
        SELECT EXISTS (
            SELECT 1 FROM users 
            WHERE id = $1
        )
    `, req.UserID).Scan(&userExists)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify user"})
		return
	}

	if !userExists {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Check if attendance already exists for this user and lesson
	var existingAttendance bool
	err = h.db.QueryRow(`
        SELECT EXISTS (
            SELECT 1 FROM attendances 
            WHERE lesson_id = $1 AND user_id = $2
        )
    `, req.LessonID, req.UserID).Scan(&existingAttendance)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check existing attendance"})
		return
	}

	if existingAttendance {
		// Update existing attendance
		var attendance models.AttendanceResponse
		err = h.db.QueryRow(`
            UPDATE attendances 
            SET status = $1, created_at = CURRENT_DATE
            WHERE lesson_id = $2 AND user_id = $3
            RETURNING id, lesson_id, user_id, status, created_at
        `, req.Status, req.LessonID, req.UserID).Scan(
			&attendance.ID,
			&attendance.LessonID,
			&attendance.UserID,
			&attendance.Status,
			&attendance.CreatedAt,
		)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update attendance"})
			return
		}

		// Get username
		err = h.db.QueryRow(`
            SELECT username FROM users WHERE id = $1
        `, attendance.UserID).Scan(&attendance.Username)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch username"})
			return
		}

		c.JSON(http.StatusOK, attendance)
		return
	}

	// Create new attendance
	var attendance models.AttendanceResponse
	err = h.db.QueryRow(`
        INSERT INTO attendances (lesson_id, user_id, status, created_at)
        VALUES ($1, $2, $3, CURRENT_DATE)
        RETURNING id, lesson_id, user_id, status, created_at
    `, req.LessonID, req.UserID, req.Status).Scan(
		&attendance.ID,
		&attendance.LessonID,
		&attendance.UserID,
		&attendance.Status,
		&attendance.CreatedAt,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create attendance"})
		return
	}

	// Get username
	err = h.db.QueryRow(`
        SELECT username FROM users WHERE id = $1
    `, attendance.UserID).Scan(&attendance.Username)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch username"})
		return
	}

	c.JSON(http.StatusCreated, attendance)
}

func (h *AttendanceHandler) GetAttendances(c *gin.Context) {
	userID := c.GetInt("userID")

	// Check if user has permission
	hasPermission, err := h.checkPermission(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify permissions"})
		return
	}

	if !hasPermission {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only Admins, Head Unicorns, and Helper Unicorns can view attendance"})
		return
	}

	lessonID := c.Query("lesson_id")

	query := `
        SELECT 
            a.id,
            a.lesson_id,
            a.user_id,
            a.status,
            a.created_at,
            u.username,
            l.title as lesson_title,
            c.name as course_name
        FROM attendances a
        JOIN users u ON u.id = a.user_id
        JOIN lessons l ON l.id = a.lesson_id
        JOIN courses c ON c.id = l.course_id
    `
	params := []interface{}{}

	if lessonID != "" {
		query += " WHERE a.lesson_id = $1"
		params = append(params, lessonID)
	}

	query += " ORDER BY a.created_at DESC"

	rows, err := h.db.Query(query, params...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch attendance records"})
		return
	}
	defer rows.Close()

	var attendances []struct {
		models.AttendanceResponse
		LessonTitle string `json:"lesson_title"`
		CourseName  string `json:"course_name"`
	}

	for rows.Next() {
		var attendance struct {
			models.AttendanceResponse
			LessonTitle string `json:"lesson_title"`
			CourseName  string `json:"course_name"`
		}
		err := rows.Scan(
			&attendance.ID,
			&attendance.LessonID,
			&attendance.UserID,
			&attendance.Status,
			&attendance.CreatedAt,
			&attendance.Username,
			&attendance.LessonTitle,
			&attendance.CourseName,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to scan attendance"})
			return
		}
		attendances = append(attendances, attendance)
	}

	c.JSON(http.StatusOK, attendances)
}

func (h *AttendanceHandler) DeleteAttendance(c *gin.Context) {
	userID := c.GetInt("userID")
	attendanceID := c.Param("id")

	// Check if user has permission
	hasPermission, err := h.checkPermission(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify permissions"})
		return
	}

	if !hasPermission {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only Admins, Head Unicorns, and Helper Unicorns can delete attendance"})
		return
	}

	// First check if attendance exists
	var exists bool
	err = h.db.QueryRow(`
        SELECT EXISTS (
            SELECT 1 FROM attendances 
            WHERE id = $1
        )
    `, attendanceID).Scan(&exists)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify attendance"})
		return
	}

	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Attendance record not found"})
		return
	}

	// Delete the attendance record
	result, err := h.db.Exec(`
        DELETE FROM attendances 
        WHERE id = $1
    `, attendanceID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete attendance"})
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify deletion"})
		return
	}

	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Attendance record not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Attendance record deleted successfully",
	})
}
