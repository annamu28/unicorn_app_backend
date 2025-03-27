package handlers

import (
	"database/sql"
	"fmt"
	"net/http"
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

	// Create chatboard
	var chatboardID int
	err = tx.QueryRow(`
        INSERT INTO chatboards (title, description)
        VALUES ($1, $2)
        RETURNING id`,
		req.Title, req.Description,
	).Scan(&chatboardID)

	if err != nil {
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
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add country access"})
			return
		}
	}

	// Commit transaction
	if err = tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
		return
	}

	// Get the created chatboard with access info
	response, err := h.getChatboardInfo(chatboardID)
	if err != nil {
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
        SELECT DISTINCT 
            cb.id,
            cb.title,
            cb.description,
            cb.created_at,
            ARRAY_AGG(DISTINCT s.name) FILTER (WHERE s.name IS NOT NULL) as squad_names,
            ARRAY_AGG(DISTINCT r.role) FILTER (WHERE r.role IS NOT NULL) as role_names,
            ARRAY_AGG(DISTINCT co.name) FILTER (WHERE co.name IS NOT NULL) as country_names
        FROM chatboards cb
        LEFT JOIN chatboard_squads cbs ON cb.id = cbs.chatboard_id
        LEFT JOIN squads s ON cbs.squad_id = s.id
        LEFT JOIN chatboard_roles cbr ON cb.id = cbr.chatboard_id
        LEFT JOIN roles r ON cbr.role_id = r.id
        LEFT JOIN squad_roles sr ON sr.role_id = r.id AND sr.squad_id = cbs.squad_id
        LEFT JOIN chatboard_countries cbc ON cb.id = cbc.chatboard_id
        LEFT JOIN countries co ON cbc.country_id = co.id
        LEFT JOIN user_squads us ON us.squad_id = s.id AND us.user_id = $1
        LEFT JOIN user_roles ur ON ur.role_id = r.id AND ur.user_id = $1
        LEFT JOIN user_countries uc ON uc.country_id = co.id AND uc.user_id = $1
        WHERE 1=1
    `

	params := []interface{}{userID}
	paramCount := 1

	// If both role and squad filters are provided, ensure they match together
	if filterRole != "" && filterSquad != "" {
		paramCount++
		paramCount++
		query += fmt.Sprintf(`
            AND EXISTS (
                SELECT 1 
                FROM chatboard_squads cbs2
                JOIN squads s2 ON s2.id = cbs2.squad_id
                JOIN squad_roles sr2 ON sr2.squad_id = s2.id
                JOIN roles r2 ON r2.id = sr2.role_id
                WHERE cbs2.chatboard_id = cb.id
                AND s2.name = $%d
                AND r2.role = $%d
            )`, paramCount-1, paramCount)
		params = append(params, filterSquad, filterRole)
	} else {
		// Handle individual filters
		if filterSquad != "" {
			paramCount++
			query += fmt.Sprintf(" AND s.name = $%d", paramCount)
			params = append(params, filterSquad)
		}

		if filterRole != "" {
			paramCount++
			query += fmt.Sprintf(" AND r.role = $%d AND sr.squad_id IS NOT NULL", paramCount)
			params = append(params, filterRole)
		}
	}

	if filterCountry != "" {
		paramCount++
		query += fmt.Sprintf(" AND co.name = $%d", paramCount)
		params = append(params, filterCountry)
	}

	query += `
        GROUP BY cb.id, cb.title, cb.description, cb.created_at
        ORDER BY cb.created_at DESC
    `

	rows, err := h.db.Query(query, params...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch chatboards"})
		return
	}
	defer rows.Close()

	var chatboards []models.ChatboardResponse
	for rows.Next() {
		var cb models.ChatboardResponse
		var squads, roles, countries []sql.NullString

		err := rows.Scan(
			&cb.ID,
			&cb.Title,
			&cb.Description,
			&cb.CreatedAt,
			pq.Array(&squads),
			pq.Array(&roles),
			pq.Array(&countries),
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to scan chatboard"})
			return
		}

		// Process arrays and remove null values
		cb.Access.Squads = nullStringArrayToStringArray(squads)
		cb.Access.Roles = nullStringArrayToStringArray(roles)
		cb.Access.Countries = nullStringArrayToStringArray(countries)

		chatboards = append(chatboards, cb)
	}

	c.JSON(http.StatusOK, chatboards)
}

func nullStringArrayToStringArray(nullStrings []sql.NullString) []string {
	var result []string
	for _, ns := range nullStrings {
		if ns.Valid {
			result = append(result, ns.String)
		}
	}
	return result
}
