package handlers

import (
	"database/sql"
	"net/http"
	"unicorn_app_backend/models"

	"github.com/gin-gonic/gin"
)

type CommentHandler struct {
	db *sql.DB
}

func NewCommentHandler(db *sql.DB) *CommentHandler {
	return &CommentHandler{db: db}
}

func (h *CommentHandler) CreateComment(c *gin.Context) {
	userID := c.GetInt("userID")
	var req models.CreateCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// First get the chatboard ID for this post
	var chatboardID int
	err := h.db.QueryRow(`
        SELECT chatboard_id 
        FROM posts 
        WHERE id = $1
    `, req.PostID).Scan(&chatboardID)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Post not found"})
		return
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch post"})
		return
	}

	// Check if user has access to the chatboard and get their role
	var hasAccess bool
	var userRole string
	err = h.db.QueryRow(`
        WITH user_access AS (
            SELECT DISTINCT r.role
            FROM chatboards cb
            LEFT JOIN chatboard_squads cbs ON cb.id = cbs.chatboard_id
            LEFT JOIN user_squads us ON us.squad_id = cbs.squad_id
            LEFT JOIN chatboard_roles cbr ON cb.id = cbr.chatboard_id
            LEFT JOIN user_roles ur ON ur.role_id = cbr.role_id
            LEFT JOIN roles r ON r.id = ur.role_id
            LEFT JOIN chatboard_countries cbc ON cb.id = cbc.chatboard_id
            LEFT JOIN user_countries uc ON uc.country_id = cbc.country_id
            WHERE cb.id = $1 
            AND us.user_id = $2
            AND (
                (us.squad_id IS NOT NULL AND cbs.squad_id IS NOT NULL)
                OR (ur.role_id IS NOT NULL AND cbr.role_id IS NOT NULL)
                OR (uc.country_id IS NOT NULL AND cbc.country_id IS NOT NULL)
            )
            LIMIT 1
        )
        SELECT EXISTS (SELECT 1 FROM user_access), 
               (SELECT role FROM user_access)
    `, chatboardID, userID).Scan(&hasAccess, &userRole)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify access"})
		return
	}

	if !hasAccess {
		c.JSON(http.StatusForbidden, gin.H{"error": "You don't have access to this chatboard"})
		return
	}

	// Create the comment
	var commentID int
	err = h.db.QueryRow(`
        INSERT INTO comments (post_id, user_id, comment, created_at)
        VALUES ($1, $2, $3, CURRENT_DATE)
        RETURNING id
    `, req.PostID, userID, req.Comment).Scan(&commentID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create comment"})
		return
	}

	// Get the complete comment information
	var comment models.CommentResponse
	err = h.db.QueryRow(`
        SELECT 
            c.id,
            c.post_id,
            c.user_id,
            c.comment,
            c.created_at,
            u.username
        FROM comments c
        JOIN users u ON u.id = c.user_id
        WHERE c.id = $1
    `, commentID).Scan(
		&comment.ID,
		&comment.PostID,
		&comment.UserID,
		&comment.Comment,
		&comment.CreatedAt,
		&comment.Author,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch comment details"})
		return
	}

	comment.UserRole = userRole

	c.JSON(http.StatusCreated, comment)
}

func (h *CommentHandler) GetComments(c *gin.Context) {
	userID := c.GetInt("userID")
	postID := c.Query("post_id")

	if postID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "post_id query parameter is required"})
		return
	}

	// First get the chatboard ID for this post
	var chatboardID int
	err := h.db.QueryRow(`
        SELECT chatboard_id 
        FROM posts 
        WHERE id = $1
    `, postID).Scan(&chatboardID)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Post not found"})
		return
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch post"})
		return
	}

	// Check if user has access to the chatboard and get their role
	var hasAccess bool
	err = h.db.QueryRow(`
        SELECT EXISTS (
            SELECT 1 
            FROM chatboards cb
            LEFT JOIN chatboard_squads cbs ON cb.id = cbs.chatboard_id
            LEFT JOIN user_squads us ON us.squad_id = cbs.squad_id
            LEFT JOIN chatboard_roles cbr ON cb.id = cbr.chatboard_id
            LEFT JOIN user_roles ur ON ur.role_id = cbr.role_id
            LEFT JOIN chatboard_countries cbc ON cb.id = cbc.chatboard_id
            LEFT JOIN user_countries uc ON uc.country_id = cbc.country_id
            WHERE cb.id = $1 
            AND us.user_id = $2
            AND (
                (us.squad_id IS NOT NULL AND cbs.squad_id IS NOT NULL)
                OR (ur.role_id IS NOT NULL AND cbr.role_id IS NOT NULL)
                OR (uc.country_id IS NOT NULL AND cbc.country_id IS NOT NULL)
            )
        )
    `, chatboardID, userID).Scan(&hasAccess)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify access"})
		return
	}

	if !hasAccess {
		c.JSON(http.StatusForbidden, gin.H{"error": "You don't have access to this chatboard"})
		return
	}

	// Get all comments for the post with user information and roles
	rows, err := h.db.Query(`
        SELECT 
            c.id,
            c.post_id,
            c.user_id,
            c.comment,
            c.created_at,
            u.username,
            r.role
        FROM comments c
        JOIN users u ON u.id = c.user_id
        LEFT JOIN user_roles ur ON ur.user_id = u.id
        LEFT JOIN roles r ON r.id = ur.role_id
        LEFT JOIN posts p ON p.id = c.post_id
        LEFT JOIN chatboard_roles cbr ON cbr.chatboard_id = p.chatboard_id AND cbr.role_id = r.id
        WHERE c.post_id = $1
        ORDER BY c.created_at ASC
    `, postID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch comments"})
		return
	}
	defer rows.Close()

	var comments []models.CommentResponse
	for rows.Next() {
		var comment models.CommentResponse
		var role sql.NullString
		err := rows.Scan(
			&comment.ID,
			&comment.PostID,
			&comment.UserID,
			&comment.Comment,
			&comment.CreatedAt,
			&comment.Author,
			&role,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to scan comment"})
			return
		}

		if role.Valid {
			comment.UserRole = role.String
		}

		comments = append(comments, comment)
	}

	c.JSON(http.StatusOK, comments)
}
