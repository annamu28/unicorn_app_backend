package handlers

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"

	"unicorn_app_backend/models"

	"github.com/gin-gonic/gin"
	"github.com/lib/pq"
)

type AvatarHandler struct {
	db *sql.DB
}

func NewAvatarHandler(db *sql.DB) *AvatarHandler {
	return &AvatarHandler{db: db}
}

// GetUserAvatar retrieves all user-related information
func (h *AvatarHandler) GetUserAvatar(c *gin.Context) {
	userID := c.GetInt("userID")

	profile, err := h.getUserProfile(userID)
	if err != nil {
		log.Printf("Error getting user profile: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch user info"})
		return
	}

	c.JSON(http.StatusOK, profile)
}

// CreateUserAvatar handles setting up user's profile information
func (h *AvatarHandler) CreateUserAvatar(c *gin.Context) {
	userID := c.GetInt("userID")
	var req models.CreateAvatarRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tx, err := h.db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction"})
		return
	}
	defer tx.Rollback()

	// Update username
	if req.Username != "" {
		_, err = tx.Exec(`
			UPDATE users 
			SET username = $1 
			WHERE id = $2
		`, req.Username, userID)
		if err != nil {
			log.Printf("Error updating username: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update username"})
			return
		}
	}

	// Process squad roles
	for _, squadRole := range req.SquadRoles {
		// Insert/update user_squads
		_, err = tx.Exec(`
			INSERT INTO user_squads (user_id, squad_id, status)
			VALUES ($1, $2, $3)
			ON CONFLICT (user_id, squad_id) 
			DO UPDATE SET status = EXCLUDED.status
		`, userID, squadRole.SquadID, squadRole.Status)
		if err != nil {
			log.Printf("Error saving user squad: %v", err)
			continue
		}

		// Check if role is Admin
		var isAdmin bool
		err = tx.QueryRow(`
			SELECT role = 'Admin' 
			FROM roles 
			WHERE id = $1
		`, squadRole.RoleID).Scan(&isAdmin)
		if err != nil {
			log.Printf("Error checking if role is admin: %v", err)
			continue
		}

		if isAdmin {
			// For Admin role, add to user_roles
			_, err = tx.Exec(`
				INSERT INTO user_roles (user_id, role_id)
				VALUES ($1, $2)
				ON CONFLICT (user_id, role_id) DO NOTHING
			`, userID, squadRole.RoleID)
		} else {
			// For non-Admin roles, add to user_squad_roles
			_, err = tx.Exec(`
				INSERT INTO user_squad_roles (user_id, squad_id, role_id)
				VALUES ($1, $2, $3)
				ON CONFLICT (user_id, squad_id, role_id) DO NOTHING
			`, userID, squadRole.SquadID, squadRole.RoleID)
		}
		if err != nil {
			log.Printf("Error saving role assignment: %v", err)
			continue
		}
	}

	// Save country
	if req.CountryID > 0 {
		_, err = tx.Exec(`
			INSERT INTO user_countries (user_id, country_id)
			VALUES ($1, $2)
			ON CONFLICT (user_id, country_id) DO NOTHING
		`, userID, req.CountryID)
		if err != nil {
			log.Printf("Error saving user country: %v", err)
		}
	}

	if err = tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save avatar"})
		return
	}

	// Get updated profile
	profile, err := h.getUserProfile(userID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "Avatar saved successfully, but failed to fetch updated profile"})
		return
	}

	c.JSON(http.StatusOK, profile)
}

// Add this method to AvatarHandler
func (h *AvatarHandler) getUserProfile(userID int) (models.UserAvatarResponse, error) {
	profile := models.UserAvatarResponse{
		Roles:     make([]string, 0),
		Squads:    make([]models.UserSquad, 0),
		Countries: make([]string, 0),
	}

	// Get username
	var username sql.NullString
	err := h.db.QueryRow(`
		SELECT username FROM users WHERE id = $1
	`, userID).Scan(&username)
	if err != nil {
		if err == sql.ErrNoRows {
			return profile, nil // Return empty profile if user not found
		}
		return profile, fmt.Errorf("failed to fetch username: %w", err)
	}
	if username.Valid {
		profile.Username = username.String
	}

	// Get global roles (including Admin)
	roleRows, err := h.db.Query(`
		SELECT DISTINCT r.role 
		FROM user_roles ur
		JOIN roles r ON r.id = ur.role_id
		WHERE ur.user_id = $1
	`, userID)
	if err != nil {
		return profile, fmt.Errorf("failed to fetch roles: %w", err)
	}
	defer roleRows.Close()

	for roleRows.Next() {
		var role string
		if err := roleRows.Scan(&role); err == nil {
			profile.Roles = append(profile.Roles, role)
		}
	}

	// Get squads with their roles
	squadRows, err := h.db.Query(`
		SELECT 
			s.id,
			s.name,
			us.status,
			COALESCE(ARRAY_AGG(DISTINCT r.role) FILTER (WHERE r.role IS NOT NULL), ARRAY[]::VARCHAR[]) as roles
		FROM user_squads us
		JOIN squads s ON s.id = us.squad_id
		LEFT JOIN user_squad_roles usr ON usr.squad_id = s.id AND usr.user_id = us.user_id
		LEFT JOIN roles r ON r.id = usr.role_id
		WHERE us.user_id = $1
		GROUP BY s.id, s.name, us.status
		ORDER BY s.name
	`, userID)
	if err != nil {
		return profile, fmt.Errorf("failed to fetch squads: %w", err)
	}
	defer squadRows.Close()

	for squadRows.Next() {
		var squad models.UserSquad
		var roles []sql.NullString
		if err := squadRows.Scan(&squad.ID, &squad.Name, &squad.Status, pq.Array(&roles)); err != nil {
			return profile, fmt.Errorf("failed to scan squad: %w", err)
		}

		squad.Roles = make([]string, 0)
		for _, role := range roles {
			if role.Valid {
				squad.Roles = append(squad.Roles, role.String)
			}
		}
		profile.Squads = append(profile.Squads, squad)
	}

	// Get countries
	countryRows, err := h.db.Query(`
		SELECT c.name 
		FROM user_countries uc
		JOIN countries c ON c.id = uc.country_id
		WHERE uc.user_id = $1
		ORDER BY c.name
	`, userID)
	if err != nil {
		return profile, fmt.Errorf("failed to fetch countries: %w", err)
	}
	defer countryRows.Close()

	for countryRows.Next() {
		var country string
		if err := countryRows.Scan(&country); err == nil {
			profile.Countries = append(profile.Countries, country)
		}
	}

	return profile, nil
}

// Add this method to AvatarHandler
func (h *AvatarHandler) VerifyUserSquad(c *gin.Context) {
	// Get the admin's userID from the context
	adminID := c.GetInt("userID")

	// Parse request
	var req models.VerificationRequest
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

	// Check if the admin has permission (must be Admin or Head Unicorn)
	var hasPermission bool
	err = tx.QueryRow(`
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
		c.JSON(http.StatusForbidden, gin.H{"error": "You don't have permission to verify users"})
		return
	}

	// Check if the user-squad combination exists
	var exists bool
	err = tx.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM user_squads
			WHERE user_id = $1 AND squad_id = $2
		)
	`, req.UserID, req.SquadID).Scan(&exists)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check user-squad existence"})
		return
	}

	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "User is not associated with this squad"})
		return
	}

	// Update the status
	_, err = tx.Exec(`
		UPDATE user_squads 
		SET status = $1
		WHERE user_id = $2 AND squad_id = $3
	`, req.Status, req.UserID, req.SquadID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update status"})
		return
	}

	// Log the verification action
	_, err = tx.Exec(`
		INSERT INTO verification_logs (
			admin_id,
			user_id,
			squad_id,
			status,
			verified_at
		) VALUES ($1, $2, $3, $4, CURRENT_TIMESTAMP)
	`, adminID, req.UserID, req.SquadID, req.Status)

	if err != nil {
		log.Printf("Failed to log verification action: %v", err)
		// Continue even if logging fails
	}

	// Commit transaction
	if err = tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit changes"})
		return
	}

	// Get updated profile
	profile, err := h.getUserProfile(req.UserID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "Status updated successfully, but failed to fetch updated profile"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("User status updated to %s", req.Status),
		"profile": profile,
	})
}
