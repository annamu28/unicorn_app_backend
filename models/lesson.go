package models

import "time"

type CreateLessonRequest struct {
	CourseID    int    `json:"course_id" binding:"required"`
	Title       string `json:"title" binding:"required"`
	Description string `json:"description" binding:"required"`
}

type LessonResponse struct {
	ID          int       `json:"id"`
	CourseID    int       `json:"course_id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}
