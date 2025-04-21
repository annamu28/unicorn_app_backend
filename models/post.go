package models

import "time"

type CreatePostRequest struct {
	ChatboardID int    `json:"chatboard_id" binding:"required"`
	Title       string `json:"title" binding:"required"`
	Content     string `json:"content" binding:"required"`
}

type PostResponse struct {
	ID           int       `json:"id"`
	ChatboardID  int       `json:"chatboard_id"`
	UserID       int       `json:"user_id"`
	Title        string    `json:"title"`
	Content      string    `json:"content"`
	CreatedAt    time.Time `json:"created_at"`
	Author       string    `json:"author"` // username of the post creator
	CommentCount int       `json:"comment_count"`
	UserRole     string    `json:"user_role"` // Add this field
	Pinned       bool      `json:"pinned"`
}

type Post struct {
	ID        int       `json:"id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	Pinned    bool      `json:"pinned"`
	CreatedAt time.Time `json:"created_at"`
	Author    Author    `json:"author"`
}

type Author struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
}
