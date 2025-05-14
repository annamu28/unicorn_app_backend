package handlers

import (
	"database/sql"
	"log"
	"net/http"
	"unicorn_app_backend/middleware"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	db           *sql.DB
	tokenService *middleware.TokenService
}

func NewAuthHandler(db *sql.DB, jwtSecret []byte) *AuthHandler {
	return &AuthHandler{
		db:           db,
		tokenService: middleware.NewTokenService(db, jwtSecret),
	}
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req struct {
		Email     string `json:"email" binding:"required,email"`
		Password  string `json:"password" binding:"required,min=8"`
		Username  string `json:"username"`
		FirstName string `json:"first_name" binding:"required"`
		LastName  string `json:"last_name" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("Registration validation error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	log.Printf("Registration attempt for email: %s", req.Email)

	var exists bool
	if err := h.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)`, req.Email).Scan(&exists); err != nil {
		log.Printf("Error checking email existence: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check email"})
		return
	}

	if exists {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Email already registered"})
		return
	}

	hashedPassword, err := middleware.HashPassword(req.Password)
	if err != nil {
		log.Printf("Error hashing password: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process password"})
		return
	}

	var userID int
	var err2 error
	if req.Username != "" {
		err2 = h.db.QueryRow(
			`INSERT INTO users (email, password_hash, username, first_name, last_name) 
			VALUES ($1, $2, $3, $4, $5) RETURNING id`,
			req.Email, hashedPassword, req.Username, req.FirstName, req.LastName,
		).Scan(&userID)
	} else {
		err2 = h.db.QueryRow(
			`INSERT INTO users (email, password_hash, first_name, last_name) 
			VALUES ($1, $2, $3, $4) RETURNING id`,
			req.Email, hashedPassword, req.FirstName, req.LastName,
		).Scan(&userID)
	}

	if err2 != nil {
		log.Printf("Error creating user: %v", err2)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	// Get user profile
	profile, err := h.getUserProfile(userID)
	if err != nil {
		log.Printf("Error getting user profile: %v", err)
	}

	// Generate tokens
	tokens, err := h.tokenService.GenerateTokens(userID)
	if err != nil {
		log.Printf("Error generating tokens: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate tokens"})
		return
	}

	log.Printf("Registration successful for user ID: %d", userID)

	// Construct complete response
	response := gin.H{
		"access_token":  tokens["access_token"],
		"refresh_token": tokens["refresh_token"],
		"user_id":       userID,
		"first_name":    req.FirstName,
		"last_name":     req.LastName,
		"email":         req.Email,
		"profile":       profile,
	}

	c.JSON(http.StatusCreated, response)
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req struct {
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	log.Printf("Login attempt for email: %s", req.Email)

	// Get basic user info
	var user struct {
		ID        int
		Email     string
		FirstName string
		LastName  string
		Username  string
	}

	var hashedPassword string
	err := h.db.QueryRow(`
		SELECT id, email, first_name, last_name, username, password_hash 
		FROM users 
		WHERE email = $1
	`, req.Email).Scan(&user.ID, &user.Email, &user.FirstName, &user.LastName, &user.Username, &hashedPassword)

	if err == sql.ErrNoRows {
		log.Printf("No user found with email: %s", req.Email)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	} else if err != nil {
		log.Printf("Error querying user: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify credentials"})
		return
	}

	log.Printf("Found user with ID: %d. Verifying password...", user.ID)
	passwordValid := middleware.VerifyPassword(hashedPassword, req.Password)
	log.Printf("Password verification result: %v", passwordValid)

	if !passwordValid {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	// Get user profile
	profile, err := h.getUserProfile(user.ID)
	if err != nil {
		log.Printf("Error getting user profile: %v", err)
	}

	// Generate tokens
	tokens, err := h.tokenService.GenerateTokens(user.ID)
	if err != nil {
		log.Printf("Error generating tokens: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate tokens"})
		return
	}

	log.Printf("Login successful for user ID: %d", user.ID)

	// Construct complete response
	response := gin.H{
		"access_token":  tokens["access_token"],
		"refresh_token": tokens["refresh_token"],
		"user_id":       user.ID,
		"first_name":    user.FirstName,
		"last_name":     user.LastName,
		"email":         user.Email,
		"profile":       profile,
	}

	c.JSON(http.StatusOK, response)
}

// getUserProfile fetches all related information for a user
func (h *AuthHandler) getUserProfile(userID int) (gin.H, error) {
	profile := gin.H{
		"username":  "",
		"roles":     []string{},
		"squads":    []gin.H{},
		"countries": []string{},
	}

	// Get username
	var username sql.NullString
	err := h.db.QueryRow(`
		SELECT username FROM users WHERE id = $1
	`, userID).Scan(&username)
	if err != nil && err != sql.ErrNoRows {
		return profile, err
	}
	if username.Valid {
		profile["username"] = username.String
	}

	// Get global roles
	roleRows, err := h.db.Query(`
		SELECT DISTINCT r.role 
		FROM user_roles ur
		JOIN roles r ON r.id = ur.role_id
		WHERE ur.user_id = $1
	`, userID)
	if err != nil && err != sql.ErrNoRows {
		return profile, err
	}

	if roleRows != nil {
		defer roleRows.Close()
		roles := []string{}
		for roleRows.Next() {
			var role string
			if err := roleRows.Scan(&role); err == nil {
				roles = append(roles, role)
			}
		}
		profile["roles"] = roles
	}

	// Get squads
	squadRows, err := h.db.Query(`
		SELECT 
			s.id,
			s.name,
			us.status
		FROM user_squads us
		JOIN squads s ON s.id = us.squad_id
		WHERE us.user_id = $1
	`, userID)
	if err != nil && err != sql.ErrNoRows {
		return profile, err
	}

	if squadRows != nil {
		defer squadRows.Close()
		squads := []gin.H{}
		for squadRows.Next() {
			var id int
			var name, status string
			if err := squadRows.Scan(&id, &name, &status); err == nil {
				squads = append(squads, gin.H{
					"id":     id,
					"name":   name,
					"status": status,
				})
			}
		}
		profile["squads"] = squads
	}

	// Get countries
	countryRows, err := h.db.Query(`
		SELECT c.name 
		FROM user_countries uc
		JOIN countries c ON c.id = uc.country_id
		WHERE uc.user_id = $1
	`, userID)
	if err != nil && err != sql.ErrNoRows {
		return profile, err
	}

	if countryRows != nil {
		defer countryRows.Close()
		countries := []string{}
		for countryRows.Next() {
			var country string
			if err := countryRows.Scan(&country); err == nil {
				countries = append(countries, country)
			}
		}
		profile["countries"] = countries
	}

	return profile, nil
}

func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID, err := h.tokenService.ValidateRefreshToken(req.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid refresh token"})
		return
	}

	tokens, err := h.tokenService.GenerateTokens(userID)
	if err != nil {
		log.Printf("Error generating tokens: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate tokens"})
		return
	}

	if err := h.tokenService.InvalidateRefreshToken(req.RefreshToken); err != nil {
		log.Printf("Error invalidating old refresh token: %v", err)
	}

	c.JSON(http.StatusOK, tokens)
}

func (h *AuthHandler) Logout(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("Logout error: missing refresh token: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Refresh token is required"})
		return
	}

	userID := c.GetInt("userID")
	log.Printf("Logout attempt for user ID: %d", userID)

	// Validate the refresh token belongs to this user before invalidating
	storedUserID, err := h.tokenService.ValidateRefreshToken(req.RefreshToken)
	if err != nil {
		log.Printf("Error validating refresh token: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid refresh token"})
		return
	}

	// Verify the token belongs to the authenticated user
	if storedUserID != userID {
		log.Printf("Token user ID (%d) doesn't match authenticated user (%d)", storedUserID, userID)
		c.JSON(http.StatusForbidden, gin.H{"error": "Unauthorized token"})
		return
	}

	if err := h.tokenService.InvalidateRefreshToken(req.RefreshToken); err != nil {
		log.Printf("Error invalidating refresh token: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to logout"})
		return
	}

	log.Printf("Logout successful for user ID: %d", userID)
	c.JSON(http.StatusOK, gin.H{"message": "Successfully logged out"})
}
