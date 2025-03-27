package handlers

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
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
	var req models.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	// Parse birthday
	birthday, err := time.Parse("2006-01-02", req.Birthday)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid birthday format"})
		return
	}

	// Insert user into database
	var userID int
	err = h.db.QueryRow(`
		INSERT INTO users (first_name, last_name, email, password_hash, birthday)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id`,
		req.FirstName, req.LastName, req.Email, string(hashedPassword), birthday,
	).Scan(&userID)

	if err != nil {
		if err.Error() == "pq: duplicate key value violates unique constraint \"users_email_key\"" {
			c.JSON(http.StatusConflict, gin.H{"error": "Email already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	// Generate JWT token
	token := h.generateToken(userID)

	c.JSON(http.StatusOK, models.AuthResponse{Token: token})
}

// LoginRequest should only have email and password
type LoginRequest struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// Login handles user login
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get user from database
	var user struct {
		ID           int
		PasswordHash string
	}
	err := h.db.QueryRow("SELECT id, password_hash FROM users WHERE email = $1", req.Email).Scan(&user.ID, &user.PasswordHash)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	// Generate tokens
	accessToken, refreshToken, err := h.generateTokens(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate tokens"})
		return
	}

	// Get user profile
	userProfile, err := h.getUserProfile(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch user profile"})
		return
	}

	// Return everything
	c.JSON(http.StatusOK, models.LoginResponse{
		Token:        accessToken,
		RefreshToken: refreshToken,
		UserInfo:     userProfile,
	})
}

// getUserProfile fetches all related information for a user
func (h *AuthHandler) getUserProfile(userID int) (models.UserProfile, error) {
	var profile models.UserProfile

	// Get username
	err := h.db.QueryRow("SELECT username FROM users WHERE id = $1", userID).Scan(&profile.Username)
	if err != nil {
		return profile, fmt.Errorf("failed to fetch username: %w", err)
	}

	// Get all roles for the user
	roleRows, err := h.db.Query(`
		SELECT DISTINCT r.role 
		FROM roles r 
		JOIN user_roles ur ON r.id = ur.role_id 
		WHERE ur.user_id = $1`, userID)
	if err != nil {
		return profile, fmt.Errorf("failed to fetch roles: %w", err)
	}
	defer roleRows.Close()

	for roleRows.Next() {
		var role string
		if err := roleRows.Scan(&role); err != nil {
			return profile, fmt.Errorf("failed to scan role: %w", err)
		}
		profile.Roles = append(profile.Roles, role)
	}

	// Get squads with their specific roles
	squadRows, err := h.db.Query(`
		SELECT 
			s.name,
			us.status,
			ARRAY_AGG(r.role) as roles
		FROM user_squads us
		JOIN squads s ON s.id = us.squad_id
		LEFT JOIN user_roles ur ON ur.user_id = us.user_id
		LEFT JOIN roles r ON r.id = ur.role_id
		LEFT JOIN squad_roles sr ON sr.role_id = r.id AND sr.squad_id = s.id
		WHERE us.user_id = $1
		AND (sr.squad_id = s.id OR sr.squad_id IS NULL)
		GROUP BY s.name, us.status`,
		userID)
	if err != nil {
		return profile, fmt.Errorf("failed to fetch squads: %w", err)
	}
	defer squadRows.Close()

	for squadRows.Next() {
		var squad models.UserSquad
		var roles []sql.NullString
		if err := squadRows.Scan(&squad.Name, &squad.Status, pq.Array(&roles)); err != nil {
			return profile, fmt.Errorf("failed to scan squad: %w", err)
		}

		// Filter out null roles
		squad.Roles = make([]string, 0)
		for _, role := range roles {
			if role.Valid && role.String != "" {
				squad.Roles = append(squad.Roles, role.String)
			}
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

		// Check if the header has the Bearer prefix
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header must be in the format: Bearer {token}"})
			c.Abort()
			return
		}

		tokenString := parts[1]

		// Parse and validate the token
		claims := &models.Claims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			// Validate the signing method
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return h.jwtSecret, nil
		})

		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			c.Abort()
			return
		}

		if !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			c.Abort()
			return
		}

		// Set the user ID in the context
		c.Set("userID", claims.UserID)
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
	newAccessToken := h.generateToken(userID)

	// Get user profile
	userProfile, err := h.getUserProfile(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch user profile"})
		return
	}

	c.JSON(http.StatusOK, models.LoginResponse{
		Token:        newAccessToken,
		RefreshToken: req.RefreshToken, // Return same refresh token
		UserInfo:     userProfile,
	})
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
