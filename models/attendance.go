package models

import "time"

type CreateAttendanceRequest struct {
	LessonID int    `json:"lesson_id" binding:"required"`
	UserID   int    `json:"user_id" binding:"required"`
	Status   string `json:"status" binding:"required,oneof=present absent late excused"`
}

type AttendanceResponse struct {
	ID        int       `json:"id"`
	LessonID  int       `json:"lesson_id"`
	UserID    int       `json:"user_id"`
	Username  string    `json:"username"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}
