package handlers

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"time"
	"unicorn_app_backend/models"

	"github.com/gin-gonic/gin"
	"github.com/lib/pq"
)

type ChatboardHandler struct {
	db *sql.DB
}

func NewChatboardHandler(db *sql.DB) *ChatboardHandler {
	return &ChatboardHandler{db: db}
}

func (h *ChatboardHandler) CreateChatboard(c *gin.Context) {
	// Get user ID from context
	userID := c.GetInt("userID")

	// Check if user has permission to create chatboards
	var hasPermission bool
	err := h.db.QueryRow(`
        SELECT EXISTS (
            SELECT 1 
            FROM user_roles ur
            JOIN roles r ON r.id = ur.role_id
            WHERE ur.user_id = $1 
            AND r.role IN ('Admin', 'Head Unicorn')
        )
    `, userID).Scan(&hasPermission)

	if err != nil {
		log.Printf("Error checking permissions: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify permissions"})
		return
	}

	if !hasPermission {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only admins and head unicorns can create chatboards"})
		return
	}

	var req models.CreateChatboardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Start transaction
	tx, err := h.db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction"})
		return
	}
	defer tx.Rollback()

	// Validate squad IDs
	if len(req.Access.SquadIDs) > 0 {
		var count int
		err = tx.QueryRow(`
            SELECT COUNT(*) FROM squads WHERE id = ANY($1)
        `, pq.Array(req.Access.SquadIDs)).Scan(&count)
		if err != nil {
			log.Printf("Error validating squad IDs: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to validate squad IDs"})
			return
		}
		if count != len(req.Access.SquadIDs) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "One or more squad IDs are invalid"})
			return
		}
	}

	// Validate role IDs
	if len(req.Access.RoleIDs) > 0 {
		var count int
		err = tx.QueryRow(`
            SELECT COUNT(*) FROM roles WHERE id = ANY($1)
        `, pq.Array(req.Access.RoleIDs)).Scan(&count)
		if err != nil {
			log.Printf("Error validating role IDs: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to validate role IDs"})
			return
		}
		if count != len(req.Access.RoleIDs) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "One or more role IDs are invalid"})
			return
		}
	}

	// Validate country IDs
	if len(req.Access.CountryIDs) > 0 {
		var count int
		err = tx.QueryRow(`
            SELECT COUNT(*) FROM countries WHERE id = ANY($1)
        `, pq.Array(req.Access.CountryIDs)).Scan(&count)
		if err != nil {
			log.Printf("Error validating country IDs: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to validate country IDs"})
			return
		}
		if count != len(req.Access.CountryIDs) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "One or more country IDs are invalid"})
			return
		}
	}

	// Create chatboard
	var chatboardID int
	err = tx.QueryRow(`
        INSERT INTO chatboards (title, description)
        VALUES ($1, $2)
        RETURNING id`,
		req.Title, req.Description,
	).Scan(&chatboardID)

	if err != nil {
		log.Printf("Error creating chatboard: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create chatboard"})
		return
	}

	// Add squad access
	for _, squadID := range req.Access.SquadIDs {
		_, err = tx.Exec(`
            INSERT INTO chatboard_squads (chatboard_id, squad_id)
            VALUES ($1, $2)`,
			chatboardID, squadID)
		if err != nil {
			log.Printf("Error adding squad access: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add squad access"})
			return
		}
	}

	// Add role access
	for _, roleID := range req.Access.RoleIDs {
		_, err = tx.Exec(`
            INSERT INTO chatboard_roles (chatboard_id, role_id)
            VALUES ($1, $2)`,
			chatboardID, roleID)
		if err != nil {
			log.Printf("Error adding role access: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add role access"})
			return
		}
	}

	// Add country access
	for _, countryID := range req.Access.CountryIDs {
		_, err = tx.Exec(`
            INSERT INTO chatboard_countries (chatboard_id, country_id)
            VALUES ($1, $2)`,
			chatboardID, countryID)
		if err != nil {
			log.Printf("Error adding country access: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add country access"})
			return
		}
	}

	// Commit transaction
	if err = tx.Commit(); err != nil {
		log.Printf("Error committing transaction: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
		return
	}

	// Get the created chatboard with access info
	response, err := h.getChatboardInfo(chatboardID)
	if err != nil {
		log.Printf("Error fetching chatboard info: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch chatboard info"})
		return
	}

	c.JSON(http.StatusCreated, response)
}

func (h *ChatboardHandler) getChatboardInfo(chatboardID int) (models.ChatboardResponse, error) {
	var response models.ChatboardResponse
	var createdAt sql.NullTime

	err := h.db.QueryRow(`
        SELECT id, title, description, created_at
        FROM chatboards
        WHERE id = $1`,
		chatboardID,
	).Scan(&response.ID, &response.Title, &response.Description, &createdAt)

	if err != nil {
		return response, err
	}

	if createdAt.Valid {
		response.CreatedAt = createdAt.Time.Format("2006-01-02")
	}

	// Get squads
	rows, err := h.db.Query(`
        SELECT s.name
        FROM chatboard_squads cs
        JOIN squads s ON s.id = cs.squad_id
        WHERE cs.chatboard_id = $1`,
		chatboardID)
	if err != nil {
		return response, err
	}
	defer rows.Close()

	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return response, err
		}
		response.Access.Squads = append(response.Access.Squads, name)
	}

	// Get roles
	rows, err = h.db.Query(`
        SELECT r.role
        FROM chatboard_roles cr
        JOIN roles r ON r.id = cr.role_id
        WHERE cr.chatboard_id = $1`,
		chatboardID)
	if err != nil {
		return response, err
	}
	defer rows.Close()

	for rows.Next() {
		var role string
		if err := rows.Scan(&role); err != nil {
			return response, err
		}
		response.Access.Roles = append(response.Access.Roles, role)
	}

	// Get countries
	rows, err = h.db.Query(`
        SELECT c.name
        FROM chatboard_countries cc
        JOIN countries c ON c.id = cc.country_id
        WHERE cc.chatboard_id = $1`,
		chatboardID)
	if err != nil {
		return response, err
	}
	defer rows.Close()

	for rows.Next() {
		var country string
		if err := rows.Scan(&country); err != nil {
			return response, err
		}
		response.Access.Countries = append(response.Access.Countries, country)
	}

	return response, nil
}

func (h *ChatboardHandler) GetChatboards(c *gin.Context) {
	userID := c.GetInt("userID")
	filterRole := c.Query("filter_role")
	filterSquad := c.Query("filter_squad")
	filterCountry := c.Query("filter_country")

	query := `
        WITH user_access AS (
            -- Get user's roles
            SELECT DISTINCT r.id as role_id, r.role as role_name
            FROM user_roles ur
            JOIN roles r ON r.id = ur.role_id
            WHERE ur.user_id = $1
            UNION
            -- Get user's squad roles
            SELECT DISTINCT r.id as role_id, r.role as role_name
            FROM user_squad_roles usr
            JOIN roles r ON r.id = usr.role_id
            WHERE usr.user_id = $1
        )
        SELECT DISTINCT 
            cb.id,
            cb.title,
            cb.description,
            cb.created_at,
            COALESCE(
                ARRAY_AGG(DISTINCT s.name) FILTER (WHERE s.name IS NOT NULL),
                ARRAY[]::VARCHAR[]
            ) as squad_names,
            COALESCE(
                ARRAY_AGG(DISTINCT r.role) FILTER (WHERE r.role IS NOT NULL),
                ARRAY[]::VARCHAR[]
            ) as role_names,
            COALESCE(
                ARRAY_AGG(DISTINCT co.name) FILTER (WHERE co.name IS NOT NULL),
                ARRAY[]::VARCHAR[]
            ) as country_names
        FROM chatboards cb
        LEFT JOIN chatboard_squads cbs ON cb.id = cbs.chatboard_id
        LEFT JOIN squads s ON cbs.squad_id = s.id
        LEFT JOIN chatboard_roles cbr ON cb.id = cbr.chatboard_id
        LEFT JOIN roles r ON cbr.role_id = r.id
        LEFT JOIN chatboard_countries cbc ON cb.id = cbc.chatboard_id
        LEFT JOIN countries co ON cbc.country_id = co.id
        LEFT JOIN user_squads us ON us.squad_id = s.id AND us.user_id = $1
        LEFT JOIN user_countries uc ON uc.country_id = co.id AND uc.user_id = $1
        WHERE (
            -- User has access through roles
            EXISTS (
                SELECT 1 FROM chatboard_roles cr
                JOIN user_access ua ON ua.role_id = cr.role_id
                WHERE cr.chatboard_id = cb.id
            )
            OR
            -- User has access through squads
            EXISTS (
                SELECT 1 FROM chatboard_squads cs
                JOIN user_squads us ON us.squad_id = cs.squad_id
                WHERE cs.chatboard_id = cb.id AND us.user_id = $1
            )
            OR
            -- User has access through countries
            EXISTS (
                SELECT 1 FROM chatboard_countries cc
                JOIN user_countries uc ON uc.country_id = cc.country_id
                WHERE cc.chatboard_id = cb.id AND uc.user_id = $1
            )
        )
    `

	params := []interface{}{userID}
	paramCount := 1

	// Add filters if provided
	if filterRole != "" {
		paramCount++
		query += fmt.Sprintf(" AND r.role = $%d", paramCount)
		params = append(params, filterRole)
	}

	if filterSquad != "" {
		paramCount++
		query += fmt.Sprintf(" AND s.name = $%d", paramCount)
		params = append(params, filterSquad)
	}

	if filterCountry != "" {
		paramCount++
		query += fmt.Sprintf(" AND co.name = $%d", paramCount)
		params = append(params, filterCountry)
	}

	query += " GROUP BY cb.id, cb.title, cb.description, cb.created_at ORDER BY cb.created_at DESC"

	rows, err := h.db.Query(query, params...)
	if err != nil {
		log.Printf("Error fetching chatboards: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch chatboards"})
		return
	}
	defer rows.Close()

	var chatboards []models.ChatboardResponse
	for rows.Next() {
		var cb models.ChatboardResponse
		var createdAt sql.NullTime
		var squadNames, roleNames, countryNames []string

		err := rows.Scan(
			&cb.ID,
			&cb.Title,
			&cb.Description,
			&createdAt,
			pq.Array(&squadNames),
			pq.Array(&roleNames),
			pq.Array(&countryNames),
		)
		if err != nil {
			log.Printf("Error scanning chatboard row: %v", err)
			continue
		}

		if createdAt.Valid {
			cb.CreatedAt = createdAt.Time.Format("2006-01-02")
		}

		cb.Access = models.ChatboardAccessInfo{
			Squads:    squadNames,
			Roles:     roleNames,
			Countries: countryNames,
		}

		chatboards = append(chatboards, cb)
	}

	if err = rows.Err(); err != nil {
		log.Printf("Error iterating chatboard rows: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch chatboards"})
		return
	}

	c.JSON(http.StatusOK, chatboards)
}

func (h *ChatboardHandler) GetPendingUsers(c *gin.Context) {
	// Get chatboard ID from URL
	chatboardID := c.Param("id")
	if chatboardID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Chatboard ID is required"})
		return
	}

	// Get the requesting user's ID
	adminID := c.GetInt("userID")

	// Check if the user has permission (must be Admin or Head Unicorn)
	var hasPermission bool
	err := h.db.QueryRow(`
        SELECT EXISTS (
            SELECT 1 FROM user_roles ur
            JOIN roles r ON r.id = ur.role_id
            WHERE ur.user_id = $1 
            AND r.role IN ('Admin', 'Head Unicorn')
        )
    `, adminID).Scan(&hasPermission)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check permissions"})
		return
	}

	if !hasPermission {
		c.JSON(http.StatusForbidden, gin.H{"error": "You don't have permission to view pending users"})
		return
	}

	// Get all pending users for the chatboard's squads
	rows, err := h.db.Query(`
        WITH chatboard_squad_ids AS (
            SELECT DISTINCT cs.squad_id
            FROM chatboard_squads cs
            WHERE cs.chatboard_id = $1
        )
        SELECT 
            u.id,
            u.first_name,
            u.last_name,
            u.email,
            s.id as squad_id,
            s.name as squad_name,
            us.status,
            COALESCE(r.role, '') as role
        FROM users u
        JOIN user_squads us ON us.user_id = u.id
        JOIN squads s ON s.id = us.squad_id
        LEFT JOIN user_squad_roles usr ON usr.user_id = u.id AND usr.squad_id = s.id
        LEFT JOIN roles r ON r.id = usr.role_id
        WHERE s.id IN (SELECT squad_id FROM chatboard_squad_ids)
        AND us.status = 'Pending'
        ORDER BY u.first_name, u.last_name, s.name
    `, chatboardID)

	if err != nil {
		log.Printf("Error fetching pending users: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch pending users"})
		return
	}
	defer rows.Close()

	var pendingUsers []models.PendingUserResponse
	for rows.Next() {
		var user models.PendingUserResponse
		err := rows.Scan(
			&user.UserID,
			&user.FirstName,
			&user.LastName,
			&user.Email,
			&user.SquadID,
			&user.SquadName,
			&user.Status,
			&user.Role,
		)
		if err != nil {
			log.Printf("Error scanning user row: %v", err)
			continue
		}
		pendingUsers = append(pendingUsers, user)
	}

	if pendingUsers == nil {
		pendingUsers = make([]models.PendingUserResponse, 0)
	}

	c.JSON(http.StatusOK, gin.H{
		"pending_users": pendingUsers,
		"count":         len(pendingUsers),
	})
}

func (h *ChatboardHandler) GetChatboardByID(c *gin.Context) {
	userID := c.GetInt("userID")
	chatboardID := c.Param("id")

	// First check if the user has access to this chatboard through any means (roles, squads, or countries)
	var hasAccess bool
	err := h.db.QueryRow(`
		SELECT EXISTS (
			SELECT 1 
			FROM chatboards cb
			WHERE cb.id = $2
			AND (
				-- Check squad access
				EXISTS (
					SELECT 1 FROM chatboard_squads cs
					JOIN user_squads us ON us.squad_id = cs.squad_id
					WHERE cs.chatboard_id = cb.id 
					AND us.user_id = $1
					AND us.status = 'approved'
				)
				OR
				-- Check role access
				EXISTS (
					SELECT 1 FROM chatboard_roles cr
					JOIN user_roles ur ON ur.role_id = cr.role_id
					WHERE cr.chatboard_id = cb.id 
					AND ur.user_id = $1
				)
				OR
				-- Check country access
				EXISTS (
					SELECT 1 FROM chatboard_countries cc
					JOIN user_countries uc ON uc.country_id = cc.country_id
					WHERE cc.chatboard_id = cb.id 
					AND uc.user_id = $1
				)
			)
		)
	`, userID, chatboardID).Scan(&hasAccess)

	if err != nil {
		log.Printf("Error checking chatboard access: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify access"})
		return
	}

	if !hasAccess {
		c.JSON(http.StatusForbidden, gin.H{"error": "You don't have access to this chatboard"})
		return
	}

	// Get chatboard details
	var chatboard struct {
		ID          int            `json:"id"`
		Title       string         `json:"title"`
		Description string         `json:"description"`
		CreatedAt   time.Time      `json:"created_at"`
		UpdatedAt   sql.NullTime   `json:"updated_at,omitempty"`
		Squads      []string       `json:"squads"`
		Roles       []string       `json:"roles"`
		Countries   []string       `json:"countries"`
		CreatorID   sql.NullInt64  `json:"creator_id,omitempty"`
		CreatorName sql.NullString `json:"creator_name,omitempty"`
	}

	// Get basic chatboard info
	err = h.db.QueryRow(`
		SELECT 
			c.id,
			c.title,
			c.description,
			c.created_at,
			c.updated_at,
			c.creator_id,
			CONCAT(u.first_name, ' ', u.last_name) as creator_name
		FROM chatboards c
		LEFT JOIN users u ON c.creator_id = u.id
		WHERE c.id = $1
	`, chatboardID).Scan(
		&chatboard.ID,
		&chatboard.Title,
		&chatboard.Description,
		&chatboard.CreatedAt,
		&chatboard.UpdatedAt,
		&chatboard.CreatorID,
		&chatboard.CreatorName,
	)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Chatboard not found"})
		return
	} else if err != nil {
		log.Printf("Error fetching chatboard: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch chatboard details"})
		return
	}

	// Get squads
	rows, err := h.db.Query(`
		SELECT s.name
		FROM chatboard_squads cs
		JOIN squads s ON s.id = cs.squad_id
		WHERE cs.chatboard_id = $1
	`, chatboardID)
	if err != nil {
		log.Printf("Error fetching squads: %v", err)
	} else {
		defer rows.Close()
		for rows.Next() {
			var squad string
			if err := rows.Scan(&squad); err == nil {
				chatboard.Squads = append(chatboard.Squads, squad)
			}
		}
	}

	// Get roles
	rows, err = h.db.Query(`
		SELECT r.role
		FROM chatboard_roles cr
		JOIN roles r ON r.id = cr.role_id
		WHERE cr.chatboard_id = $1
	`, chatboardID)
	if err != nil {
		log.Printf("Error fetching roles: %v", err)
	} else {
		defer rows.Close()
		for rows.Next() {
			var role string
			if err := rows.Scan(&role); err == nil {
				chatboard.Roles = append(chatboard.Roles, role)
			}
		}
	}

	// Get countries
	rows, err = h.db.Query(`
		SELECT c.name
		FROM chatboard_countries cc
		JOIN countries c ON c.id = cc.country_id
		WHERE cc.chatboard_id = $1
	`, chatboardID)
	if err != nil {
		log.Printf("Error fetching countries: %v", err)
	} else {
		defer rows.Close()
		for rows.Next() {
			var country string
			if err := rows.Scan(&country); err == nil {
				chatboard.Countries = append(chatboard.Countries, country)
			}
		}
	}

	// Initialize empty arrays if null
	if chatboard.Squads == nil {
		chatboard.Squads = make([]string, 0)
	}
	if chatboard.Roles == nil {
		chatboard.Roles = make([]string, 0)
	}
	if chatboard.Countries == nil {
		chatboard.Countries = make([]string, 0)
	}

	c.JSON(http.StatusOK, chatboard)
}
