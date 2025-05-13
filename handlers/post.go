package handlers

import (
	"database/sql"
	"log"
	"net/http"
	"time"
	"unicorn_app_backend/models" // replace with your actual project name

	"github.com/gin-gonic/gin"
	"github.com/lib/pq"
)

type PostHandler struct {
	db *sql.DB
}

func NewPostHandler(db *sql.DB) *PostHandler {
	return &PostHandler{db: db}
}

func (h *PostHandler) CreatePost(c *gin.Context) {
	// Get user ID from context (set by auth middleware)
	userID := c.GetInt("userID")

	// Bind the request body
	var input struct {
		ChatboardID int    `json:"chatboard_id" binding:"required"`
		Title       string `json:"title" binding:"required"`
		Content     string `json:"content" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// First, verify that the user has access to this chatboard
	var count int
	err := h.db.QueryRow(`
        SELECT COUNT(1) FROM chatboard_squads cs
        JOIN user_squads us ON cs.squad_id = us.squad_id
        WHERE cs.chatboard_id = $1 AND us.user_id = $2
    `, input.ChatboardID, userID).Scan(&count)

	if err != nil {
		log.Printf("Error checking chatboard access: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify chatboard access"})
		return
	}

	if count == 0 {
		c.JSON(http.StatusForbidden, gin.H{"error": "You don't have access to this chatboard"})
		return
	}

	// Create the post
	var postID int
	err = h.db.QueryRow(`
        INSERT INTO posts (chatboard_id, user_id, title, content, created_at)
        VALUES ($1, $2, $3, $4, CURRENT_TIMESTAMP)
        RETURNING id
    `, input.ChatboardID, userID, input.Title, input.Content).Scan(&postID)

	if err != nil {
		log.Printf("Error creating post: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create post"})
		return
	}

	// Fetch the created post
	var post models.Post
	err = h.db.QueryRow(`
        SELECT 
            p.id,
            p.title,
            p.content,
            p.pinned,
            p.created_at,
            u.first_name,
            u.last_name,
            u.email
        FROM posts p
        JOIN users u ON p.user_id = u.id
        WHERE p.id = $1
    `, postID).Scan(
		&post.ID,
		&post.Title,
		&post.Content,
		&post.Pinned,
		&post.CreatedAt,
		&post.Author.FirstName,
		&post.Author.LastName,
		&post.Author.Email,
	)

	if err != nil {
		log.Printf("Error fetching created post: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Post created but failed to fetch details"})
		return
	}

	c.JSON(http.StatusCreated, post)
}

func (h *PostHandler) GetPosts(c *gin.Context) {
	userID := c.GetInt("userID")
	chatboardID := c.Query("chatboard_id")
	if chatboardID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Chatboard ID is required"})
		return
	}

	// Check if user has access to the chatboard through any of the access methods (squads, roles, or countries)
	var hasAccess bool
	err := h.db.QueryRow(`
        SELECT EXISTS (
            SELECT 1
            FROM chatboards cb
            WHERE cb.id = $1
            AND (
                -- Check squad access
                EXISTS (
                    SELECT 1 FROM chatboard_squads cs
                    JOIN user_squads us ON cs.squad_id = us.squad_id
                    WHERE cs.chatboard_id = cb.id
                    AND us.user_id = $2
                    AND us.status = 'active'
                )
                OR
                -- Check role access
                EXISTS (
                    SELECT 1 FROM chatboard_roles cr
                    JOIN user_roles ur ON cr.role_id = ur.role_id
                    WHERE cr.chatboard_id = cb.id
                    AND ur.user_id = $2
                )
                OR
                -- Check country access
                EXISTS (
                    SELECT 1 FROM chatboard_countries cc
                    JOIN user_countries uc ON cc.country_id = uc.country_id
                    WHERE cc.chatboard_id = cb.id
                    AND uc.user_id = $2
                )
            )
    )`, chatboardID, userID).Scan(&hasAccess)

	if err != nil {
		log.Printf("Error checking access: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify access"})
		return
	}

	if !hasAccess {
		c.JSON(http.StatusForbidden, gin.H{"error": "You don't have access to this chatboard"})
		return
	}

	// Get posts with user roles in the chatboard
	rows, err := h.db.Query(`
        SELECT 
            p.id,
            p.title,
            p.content,
            p.pinned,
            p.created_at,
            u.username,
            COALESCE(
                ARRAY_AGG(DISTINCT r.role) FILTER (WHERE r.role IS NOT NULL),
                ARRAY[]::VARCHAR[]
            ) as user_roles
        FROM posts p
        JOIN users u ON p.user_id = u.id
        LEFT JOIN user_squad_roles usr ON usr.user_id = u.id
        LEFT JOIN roles r ON r.id = usr.role_id
        LEFT JOIN chatboard_roles cr ON cr.role_id = r.id AND cr.chatboard_id = p.chatboard_id
        LEFT JOIN chatboard_squads cs ON cs.chatboard_id = p.chatboard_id
        LEFT JOIN user_squads us ON us.squad_id = cs.squad_id AND us.user_id = u.id
        WHERE p.chatboard_id = $1
        GROUP BY p.id, p.title, p.content, p.pinned, p.created_at, u.username
        ORDER BY p.pinned DESC, p.created_at DESC`,
		chatboardID)
	if err != nil {
		log.Printf("Error fetching posts: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch posts"})
		return
	}
	defer rows.Close()

	var posts []gin.H
	for rows.Next() {
		var (
			id        int
			title     string
			content   string
			pinned    bool
			createdAt time.Time
			username  sql.NullString
			roles     []string
		)

		err := rows.Scan(
			&id,
			&title,
			&content,
			&pinned,
			&createdAt,
			&username,
			pq.Array(&roles),
		)
		if err != nil {
			log.Printf("Error scanning post: %v", err)
			continue
		}

		post := gin.H{
			"id":         id,
			"title":      title,
			"content":    content,
			"pinned":     pinned,
			"created_at": createdAt,
			"author": gin.H{
				"username": username.String,
				"roles":    roles,
			},
		}
		posts = append(posts, post)
	}

	if err = rows.Err(); err != nil {
		log.Printf("Error iterating posts: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error processing posts"})
		return
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
