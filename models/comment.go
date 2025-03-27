package models

import "time"

type CreateCommentRequest struct {
	PostID  int    `json:"post_id" binding:"required"`
	Comment string `json:"comment" binding:"required"`
}

type CommentResponse struct {
	ID        int       `json:"id"`
	PostID    int       `json:"post_id"`
	UserID    int       `json:"user_id"`
	Author    string    `json:"author"`
	Comment   string    `json:"comment"`
	CreatedAt time.Time `json:"created_at"`
	UserRole  string    `json:"user_role"` // Role of the commenter in the chatboard
}
