package handlers

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
	"unicorn_app_backend/models"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

type AuthHandler struct {
	db        *sql.DB
	jwtSecret []byte
}

func NewAuthHandler(db *sql.DB, jwtSecret []byte) *AuthHandler {
	return &AuthHandler{
		db:        db,
		jwtSecret: jwtSecret,
	}
}

func (h *AuthHandler) Register(c *gin.Context) {
	var input struct {
		FirstName string `json:"first_name" binding:"required"`
		LastName  string `json:"last_name" binding:"required"`
		Email     string `json:"email" binding:"required,email"`
		Password  string `json:"password" binding:"required,min=6"`
		Birthday  string `json:"birthday" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Hash the password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	// Insert the new user
	var userID int
	err = h.db.QueryRow(`
		INSERT INTO users (first_name, last_name, email, password_hash, birthday)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id`,
		input.FirstName, input.LastName, input.Email, string(hashedPassword), input.Birthday).Scan(&userID)

	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			c.JSON(http.StatusConflict, gin.H{"error": "Email already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	// Generate tokens
	accessToken, refreshToken, err := h.generateTokens(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate tokens"})
		return
	}

	// Return response with all requested fields
	c.JSON(http.StatusOK, models.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		UserID:       userID,
		FirstName:    input.FirstName,
		LastName:     input.LastName,
		Email:        input.Email,
	})
}

// LoginRequest should only have email and password
type LoginRequest struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// Login handles user login
func (h *AuthHandler) Login(c *gin.Context) {
	var input struct {
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get user from database
	var user struct {
		ID           int
		Email        string
		FirstName    string
		LastName     string
		PasswordHash string
	}

	err := h.db.QueryRow(`
		SELECT id, email, first_name, last_name, password_hash 
		FROM users 
		WHERE email = $1`,
		input.Email).Scan(&user.ID, &user.Email, &user.FirstName, &user.LastName, &user.PasswordHash)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch user"})
		return
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	// After successful password verification, fetch user profile data
	profile, err := h.getUserProfile(user.ID)
	if err != nil {
		log.Printf("Error getting user profile: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user profile"})
		return
	}

	// Generate tokens
	accessToken, refreshToken, err := h.generateTokens(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate tokens"})
		return
	}

	// Return complete response
	response := models.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		UserID:       user.ID,
		FirstName:    user.FirstName,
		LastName:     user.LastName,
		Email:        user.Email,
		Profile:      profile,
	}

	c.JSON(http.StatusOK, response)
}

// getUserProfile fetches all related information for a user
func (h *AuthHandler) getUserProfile(userID int) (models.UserProfile, error) {
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
		return profile, fmt.Errorf("failed to fetch username: %w", err)
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
			COALESCE(
				ARRAY_AGG(r.role) FILTER (WHERE r.role IS NOT NULL),
				ARRAY[]::VARCHAR[]
			) as roles
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

func (h *AuthHandler) generateToken(userID int) string {
	claims := &models.Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(365 * 24 * time.Hour)), // 1 year
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, _ := token.SignedString(h.jwtSecret)
	return tokenString
}

// Add CORS middleware to allow requests from Flutter app
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

// AuthMiddleware creates a gin middleware for JWT authentication
func (h *AuthHandler) AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header is required"})
			c.Abort()
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header must be in the format: Bearer {token}"})
			c.Abort()
			return
		}

		tokenString := parts[1]
		claims := &models.Claims{}

		// Add debug logging
		log.Printf("Validating token: %s", tokenString[:10]) // Only log first 10 chars for security

		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return h.jwtSecret, nil
		})

		if err != nil {
			log.Printf("Token validation error: %v", err)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			c.Abort()
			return
		}

		if !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			c.Abort()
			return
		}

		// Set both userID and token in context
		c.Set("userID", claims.UserID)
		c.Set("token", tokenString)

		log.Printf("Successfully authenticated user: %d", claims.UserID)
		c.Next()
	}
}

func (h *AuthHandler) generateTokens(userID int) (string, string, error) {
	// Access token - short lived (1 day)
	accessClaims := &models.Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessTokenString, _ := accessToken.SignedString(h.jwtSecret)

	// Refresh token - long lived (1 year)
	refreshToken := generateRandomString(32) // Implement this helper function

	// Store refresh token in database
	_, err := h.db.Exec(`
		INSERT INTO refresh_tokens (user_id, token, expires_at)
		VALUES ($1, $2, $3)`,
		userID,
		refreshToken,
		time.Now().Add(365*24*time.Hour),
	)

	if err != nil {
		return "", "", err
	}

	return accessTokenString, refreshToken, nil
}

// Add new refresh token endpoint
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var req RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Verify refresh token exists and is valid
	var userID int
	err := h.db.QueryRow(`
		SELECT user_id 
		FROM refresh_tokens 
		WHERE token = $1 AND expires_at > NOW()`,
		req.RefreshToken,
	).Scan(&userID)

	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid refresh token"})
		return
	}

	// Generate new access token
	accessToken, err := h.generateAccessToken(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate access token"})
		return
	}

	// Return the response using the correct struct fields
	response := models.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: req.RefreshToken,
		UserID:       userID,
	}

	c.JSON(http.StatusOK, response)
}

func generateRandomString(length int) string {
	bytes := make([]byte, length/2)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// Move the RefreshRequest type from models/auth.go to handlers/auth.go
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

func (h *AuthHandler) Logout(c *gin.Context) {
	// Get user ID from context (set by auth middleware)
	userID := c.GetInt("userID")

	// Get refresh token from request
	var req struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Refresh token is required"})
		return
	}

	// Delete the refresh token from database
	result, err := h.db.Exec(`
		DELETE FROM refresh_tokens 
		WHERE user_id = $1 AND token = $2`,
		userID, req.RefreshToken)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to logout"})
		return
	}

	// Check if any row was affected
	rowsAffected, err := result.RowsAffected()
	if err != nil || rowsAffected == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid refresh token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Successfully logged out",
	})
}

func (h *AuthHandler) verifyPassword(inputPassword, storedPasswordHash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(storedPasswordHash), []byte(inputPassword))
	return err == nil
}

func (h *AuthHandler) generateAccessToken(userID int) (string, error) {
	claims := &models.Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(h.jwtSecret)
	if err != nil {
		return "", err
	}
	return tokenString, nil
}

func (h *AuthHandler) generateRefreshToken(userID int) (string, error) {
	claims := &models.Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(365 * 24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(h.jwtSecret)
	if err != nil {
		return "", err
	}
	return tokenString, nil
}

func (h *AuthHandler) GetSquads(c *gin.Context) {
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

// Similar updates for GetCountries and GetRoles

func (h *AuthHandler) GetUserInfo(c *gin.Context) {
	userID := c.GetInt("userID")

	// Create an instance of AvatarHandler to use its getUserProfile method
	avatarHandler := &AvatarHandler{db: h.db}

	profile, err := avatarHandler.getUserProfile(userID)
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
