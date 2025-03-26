package handlers

import (
	"database/sql"
	"fmt"
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
	userID := c.GetInt("userID")
	var req models.CreateAvatarRequest
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

	// Update username
	_, err = tx.Exec("UPDATE users SET username = $1 WHERE id = $2",
		req.Username, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update username"})
		return
	}

	// Handle each squad role
	for _, squadRole := range req.SquadRoles {
		// Validate role for squad
		var isValidRole bool
		err = tx.QueryRow(`
			SELECT EXISTS (
				SELECT 1 FROM roles r
				LEFT JOIN squad_roles sr ON r.id = sr.role_id
				WHERE r.id = $1 AND (
					sr.squad_id IS NULL OR -- global role
					sr.squad_id = $2      -- squad-specific role
				)
			)
		`, squadRole.RoleID, squadRole.SquadID).Scan(&isValidRole)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to validate role"})
			return
		}

		if !isValidRole {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": fmt.Sprintf("Invalid role %d for squad %d",
					squadRole.RoleID, squadRole.SquadID),
			})
			return
		}

		// Insert or update user_roles
		_, err = tx.Exec(`
			INSERT INTO user_roles (user_id, role_id)
			VALUES ($1, $2)
			ON CONFLICT (user_id, role_id) DO NOTHING`,
			userID, squadRole.RoleID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to assign role"})
			return
		}

		// Insert or update user_squads
		_, err = tx.Exec(`
			INSERT INTO user_squads (user_id, squad_id, status)
			VALUES ($1, $2, $3)
			ON CONFLICT (user_id, squad_id) DO UPDATE SET status = $3`,
			userID, squadRole.SquadID, squadRole.Status)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to assign squad"})
			return
		}
	}

	// Handle country
	_, err = tx.Exec(`
		INSERT INTO user_countries (user_id, country_id)
		VALUES ($1, $2)
		ON CONFLICT (user_id, country_id) DO NOTHING`,
		userID, req.CountryID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to assign country"})
		return
	}

	// Commit transaction
	if err = tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit changes"})
		return
	}

	// Get updated profile
	profile, err := h.getUserProfile(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch updated profile"})
		return
	}

	c.JSON(http.StatusOK, profile)
}

// Add this method to AvatarHandler
func (h *AvatarHandler) getUserProfile(userID int) (models.UserAvatarResponse, error) {
	var profile models.UserAvatarResponse

	// Get username
	err := h.db.QueryRow("SELECT username FROM users WHERE id = $1", userID).Scan(&profile.Username)
	if err != nil {
		return profile, fmt.Errorf("failed to fetch username: %w", err)
	}

	// Get user's roles with squad information
	rows, err := h.db.Query(`
		SELECT r.role 
		FROM roles r 
		JOIN user_roles ur ON r.id = ur.role_id 
		WHERE ur.user_id = $1`, userID)
	if err != nil {
		return profile, fmt.Errorf("failed to fetch roles: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var role string
		if err := rows.Scan(&role); err != nil {
			return profile, fmt.Errorf("failed to scan role: %w", err)
		}
		profile.Roles = append(profile.Roles, role)
	}

	// Get squads with status
	squadRows, err := h.db.Query(`
		SELECT s.name, us.status 
		FROM squads s 
		JOIN user_squads us ON s.id = us.squad_id 
		WHERE us.user_id = $1`, userID)
	if err != nil {
		return profile, fmt.Errorf("failed to fetch squads: %w", err)
	}
	defer squadRows.Close()

	for squadRows.Next() {
		var squad models.UserSquad
		if err := squadRows.Scan(&squad.Name, &squad.Status); err != nil {
			return profile, fmt.Errorf("failed to scan squad: %w", err)
		}
		profile.Squads = append(profile.Squads, squad)
	}

	// Get countries
	countryRows, err := h.db.Query(`
		SELECT c.name 
		FROM countries c 
		JOIN user_countries uc ON c.id = uc.country_id 
		WHERE uc.user_id = $1`, userID)
	if err != nil {
		return profile, fmt.Errorf("failed to fetch countries: %w", err)
	}
	defer countryRows.Close()

	for countryRows.Next() {
		var country string
		if err := countryRows.Scan(&country); err != nil {
			return profile, fmt.Errorf("failed to scan country: %w", err)
		}
		profile.Countries = append(profile.Countries, country)
	}

	return profile, nil
}
