package handlers

import (
	"database/sql"
	"log"
	"net/http"
	"unicorn_app_backend/models"

	"github.com/gin-gonic/gin"
	"github.com/lib/pq"
)

type UserHandler struct {
	db *sql.DB
}

func NewUserHandler(db *sql.DB) *UserHandler {
	return &UserHandler{db: db}
}

// GetUserInfo fetches the user's profile information
func (h *UserHandler) GetUserInfo(c *gin.Context) {
	userID := c.GetInt("userID")

	profile, err := h.getUserProfile(userID)
	if err != nil {
		log.Printf("Error getting user profile: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch user info"})
		return
	}

	// Initialize empty arrays if they're nil
	if profile.Squads == nil {
		profile.Squads = []models.UserSquad{}
	}
	if profile.Roles == nil {
		profile.Roles = []string{}
	}
	if profile.Countries == nil {
		profile.Countries = []string{}
	}

	c.JSON(http.StatusOK, profile)
}

// getUserProfile fetches all related information for a user
func (h *UserHandler) getUserProfile(userID int) (models.UserProfile, error) {
	profile := models.UserProfile{
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
			return profile, nil
		}
		return profile, err
	}
	if username.Valid {
		profile.Username = username.String
	}

	// Get global roles
	roleRows, err := h.db.Query(`
		SELECT DISTINCT r.role 
		FROM user_roles ur
		JOIN roles r ON r.id = ur.role_id
		WHERE ur.user_id = $1
	`, userID)
	if err != nil {
		return profile, err
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
		WITH squad_roles AS (
			SELECT 
				usr.squad_id,
				usr.user_id,
				ARRAY_AGG(r.role) as roles
			FROM user_squad_roles usr
			JOIN roles r ON r.id = usr.role_id
			GROUP BY usr.squad_id, usr.user_id
		)
		SELECT 
			s.id,
			s.name,
			us.status,
			COALESCE(sr.roles, ARRAY[]::VARCHAR[]) as roles
		FROM user_squads us
		JOIN squads s ON s.id = us.squad_id
		LEFT JOIN squad_roles sr ON sr.squad_id = s.id AND sr.user_id = us.user_id
		WHERE us.user_id = $1
		ORDER BY s.name
	`, userID)
	if err != nil {
		return profile, err
	}
	defer squadRows.Close()

	for squadRows.Next() {
		var squad models.UserSquad
		var roles []string
		if err := squadRows.Scan(&squad.ID, &squad.Name, &squad.Status, pq.Array(&roles)); err != nil {
			return profile, err
		}
		squad.Roles = roles
		profile.Squads = append(profile.Squads, squad)
	}

	// Get countries
	countryRows, err := h.db.Query(`
		SELECT c.name 
		FROM user_countries uc
		JOIN countries c ON c.id = uc.country_id
		WHERE uc.user_id = $1
	`, userID)
	if err != nil {
		return profile, err
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

// GetSquads fetches all squads the user is a member of
func (h *UserHandler) GetSquads(c *gin.Context) {
	userID := c.GetInt("userID")

	// Query to get squads with their status for the user
	rows, err := h.db.Query(`
		SELECT 
			s.id,
			s.name,
			us.status
		FROM squads s
		JOIN user_squads us ON s.id = us.squad_id
		WHERE us.user_id = $1
		ORDER BY s.name`,
		userID)
	if err != nil {
		log.Printf("Error fetching squads: %v", err)
		c.JSON(http.StatusOK, []gin.H{}) // Return empty array instead of error
		return
	}
	defer rows.Close()

	// Slice to store the squads
	var squads []gin.H

	// Iterate through the rows
	for rows.Next() {
		var (
			id     int
			name   string
			status string
		)

		// Scan the row into variables
		if err := rows.Scan(&id, &name, &status); err != nil {
			log.Printf("Error scanning squad row: %v", err)
			continue
		}

		// Add squad to the slice
		squads = append(squads, gin.H{
			"id":     id,
			"name":   name,
			"status": status,
		})
	}

	// Check for errors during iteration
	if err = rows.Err(); err != nil {
		log.Printf("Error iterating squad rows: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch squads"})
		return
	}

	// Return the squads
	c.JSON(http.StatusOK, squads)
}
