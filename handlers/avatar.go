package handlers

import (
	"database/sql"
	"net/http"

	"unicorn_app_backend/models"

	"github.com/gin-gonic/gin"
)

type AvatarHandler struct {
	db *sql.DB
}

func NewAvatarHandler(db *sql.DB) *AvatarHandler {
	return &AvatarHandler{db: db}
}

// GetUserAvatar retrieves all user-related information
func (h *AvatarHandler) GetUserAvatar(c *gin.Context) {
	userID := c.GetInt("userID") // This should be set by your auth middleware

	// Get user's basic info
	var username string
	err := h.db.QueryRow("SELECT username FROM users WHERE id = $1", userID).Scan(&username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch user info"})
		return
	}

	// Get user's roles
	rows, err := h.db.Query(`
		SELECT r.role 
		FROM roles r 
		JOIN user_roles ur ON r.id = ur.role_id 
		WHERE ur.user_id = $1`, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch roles"})
		return
	}
	defer rows.Close()

	var roles []string
	for rows.Next() {
		var role string
		if err := rows.Scan(&role); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to scan role"})
			return
		}
		roles = append(roles, role)
	}

	// Get user's squads with status
	squadRows, err := h.db.Query(`
		SELECT s.name, us.status 
		FROM squads s 
		JOIN user_squads us ON s.id = us.squad_id 
		WHERE us.user_id = $1`, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch squads"})
		return
	}
	defer squadRows.Close()

	var squads []models.UserSquad
	for squadRows.Next() {
		var squad models.UserSquad
		if err := squadRows.Scan(&squad.Name, &squad.Status); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to scan squad"})
			return
		}
		squads = append(squads, squad)
	}

	// Get user's countries
	countryRows, err := h.db.Query(`
		SELECT c.name 
		FROM countries c 
		JOIN user_countries uc ON c.id = uc.country_id 
		WHERE uc.user_id = $1`, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch countries"})
		return
	}
	defer countryRows.Close()

	var countries []string
	for countryRows.Next() {
		var country string
		if err := countryRows.Scan(&country); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to scan country"})
			return
		}
		countries = append(countries, country)
	}

	// Return all user information
	c.JSON(http.StatusOK, models.UserAvatarResponse{
		Username:  username,
		Roles:     roles,
		Squads:    squads,
		Countries: countries,
	})
}

// CreateUserAvatar handles setting up user's profile information
func (h *AvatarHandler) CreateUserAvatar(c *gin.Context) {
	userID := c.GetInt("userID") // This should be set by your auth middleware

	var req models.AvatarRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Start a transaction
	tx, err := h.db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction"})
		return
	}
	defer tx.Rollback()

	// Update username
	_, err = tx.Exec("UPDATE users SET username = $1 WHERE id = $2",
		req.Username, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update username"})
		return
	}

	// Insert into user_roles
	_, err = tx.Exec(`
		INSERT INTO user_roles (user_id, role_id) 
		VALUES ($1, $2) 
		ON CONFLICT (user_id, role_id) DO NOTHING`,
		userID, req.RoleID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to assign role"})
		return
	}

	// Insert into user_squads with status
	_, err = tx.Exec(`
		INSERT INTO user_squads (user_id, squad_id, status) 
		VALUES ($1, $2, $3) 
		ON CONFLICT (user_id, squad_id) DO UPDATE SET status = $3`,
		userID, req.SquadID, req.Status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to assign squad"})
		return
	}

	// Insert into user_countries
	_, err = tx.Exec(`
		INSERT INTO user_countries (user_id, country_id) 
		VALUES ($1, $2) 
		ON CONFLICT (user_id, country_id) DO NOTHING`,
		userID, req.CountryID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to assign country"})
		return
	}

	// Commit the transaction
	if err = tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
		return
	}

	c.JSON(http.StatusOK, models.AvatarResponse{
		Message: "Profile information updated successfully",
	})
}
