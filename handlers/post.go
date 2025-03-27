package handlers

import (
	"database/sql"
	"net/http"
	"unicorn_app_backend/models" // replace with your actual project name

	"github.com/gin-gonic/gin"
)

type PostHandler struct {
	db *sql.DB
}

func NewPostHandler(db *sql.DB) *PostHandler {
	return &PostHandler{db: db}
}

func (h *PostHandler) CreatePost(c *gin.Context) {
	userID := c.GetInt("userID") // from auth middleware
	var req models.CreatePostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if user has access to the chatboard
	var hasAccess bool
	err := h.db.QueryRow(`
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
    `, req.ChatboardID, userID).Scan(&hasAccess)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify access"})
		return
	}

	if !hasAccess {
		c.JSON(http.StatusForbidden, gin.H{"error": "You don't have access to this chatboard"})
		return
	}

	// Create the post
	var postID int
	err = h.db.QueryRow(`
        INSERT INTO posts (chatboard_id, user_id, title, content, created_at)
        VALUES ($1, $2, $3, $4, CURRENT_DATE)
        RETURNING id, created_at
    `, req.ChatboardID, userID, req.Title, req.Content).Scan(&postID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create post"})
		return
	}

	// Get the complete post information including author username
	var post models.PostResponse
	err = h.db.QueryRow(`
        SELECT 
            p.id,
            p.chatboard_id,
            p.user_id,
            p.title,
            p.content,
            p.created_at,
            u.username
        FROM posts p
        JOIN users u ON u.id = p.user_id
        WHERE p.id = $1
    `, postID).Scan(
		&post.ID,
		&post.ChatboardID,
		&post.UserID,
		&post.Title,
		&post.Content,
		&post.CreatedAt,
		&post.Author,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch post details"})
		return
	}

	c.JSON(http.StatusCreated, post)
}

func (h *PostHandler) GetPosts(c *gin.Context) {
	userID := c.GetInt("userID")
	chatboardID := c.Query("chatboard_id")

	if chatboardID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "chatboard_id query parameter is required"})
		return
	}

	// First check if user has access and get their role
	var hasAccess bool
	var userRole string
	err := h.db.QueryRow(`
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

	// Get posts with author information, comment count, and pin status
	rows, err := h.db.Query(`
        SELECT 
            p.id,
            p.chatboard_id,
            p.user_id,
            p.title,
            p.content,
            p.created_at,
            u.username,
            COUNT(c.id) as comment_count,
            p.pinned
        FROM posts p
        JOIN users u ON u.id = p.user_id
        LEFT JOIN comments c ON c.post_id = p.id
        WHERE p.chatboard_id = $1
        GROUP BY 
            p.id, 
            p.chatboard_id, 
            p.user_id, 
            p.title, 
            p.content, 
            p.created_at, 
            u.username,
            p.pinned
        ORDER BY 
            p.pinned DESC,           -- Pinned posts first
            p.created_at DESC        -- Then by date (newest first)
    `, chatboardID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch posts"})
		return
	}
	defer rows.Close()

	var posts []models.PostResponse
	for rows.Next() {
		var post models.PostResponse
		var commentCount int
		err := rows.Scan(
			&post.ID,
			&post.ChatboardID,
			&post.UserID,
			&post.Title,
			&post.Content,
			&post.CreatedAt,
			&post.Author,
			&commentCount,
			&post.Pinned,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to scan post"})
			return
		}
		post.CommentCount = commentCount
		post.UserRole = userRole // Add the user's role for the chatboard
		posts = append(posts, post)
	}

	c.JSON(http.StatusOK, posts)
}

func (h *PostHandler) TogglePin(c *gin.Context) {
	userID := c.GetInt("userID")
	postID := c.Param("id")

	// First, get the chatboard ID and current pin status for this post
	var chatboardID int
	var currentPinned bool
	err := h.db.QueryRow(`
        SELECT chatboard_id, pinned 
        FROM posts 
        WHERE id = $1
    `, postID).Scan(&chatboardID, &currentPinned)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Post not found"})
		return
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch post"})
		return
	}

	// Check if user has admin/moderator role in this chatboard
	var hasPermission bool
	err = h.db.QueryRow(`
        SELECT EXISTS (
            SELECT 1 
            FROM chatboard_roles cbr
            JOIN user_roles ur ON ur.role_id = cbr.role_id
            JOIN roles r ON r.id = ur.role_id
            WHERE cbr.chatboard_id = $1 
            AND ur.user_id = $2
            AND r.role IN ('Admin', 'Moderator')
        )
    `, chatboardID, userID).Scan(&hasPermission)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify permissions"})
		return
	}

	if !hasPermission {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only admins and moderators can pin/unpin posts"})
		return
	}

	// Toggle the pin status
	newPinned := !currentPinned
	_, err = h.db.Exec(`
        UPDATE posts 
        SET pinned = $1 
        WHERE id = $2
    `, newPinned, postID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update pin status"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Pin status updated successfully",
		"pinned":  newPinned,
	})
}
