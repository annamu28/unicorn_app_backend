package models

import "time"

type Course struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
}

type CreateCourseRequest struct {
	Name string `json:"name" binding:"required"`
}

type CourseResponse struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
}

type CourseWithLessonsResponse struct {
	ID        int              `json:"id"`
	Name      string           `json:"name"`
	CreatedAt time.Time        `json:"created_at"`
	Lessons   []LessonResponse `json:"lessons"`
}
