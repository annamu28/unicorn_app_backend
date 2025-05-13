package middleware

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
	"golang.org/x/crypto/bcrypt"
)

// AuthMiddleware creates a gin middleware for JWT authentication
func AuthMiddleware(db *sql.DB, jwtSecret []byte) gin.HandlerFunc {
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
			return jwtSecret, nil
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

		// Get user's role
		var userRole string
		err = db.QueryRow(`
			SELECT r.role 
			FROM user_roles ur
			JOIN roles r ON r.id = ur.role_id
			WHERE ur.user_id = $1
			AND r.role IN ('Admin', 'Head Unicorn')
			LIMIT 1
		`, claims.UserID).Scan(&userRole)

		if err != nil && err != sql.ErrNoRows {
			log.Printf("Error getting user role: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user role"})
			c.Abort()
			return
		}

		// Set both userID and role in context
		c.Set("userID", claims.UserID)
		c.Set("userRole", userRole)
		c.Set("token", tokenString)

		log.Printf("Successfully authenticated user: %d with role: %s", claims.UserID, userRole)
		c.Next()
	}
}

// TokenService handles token generation and validation
type TokenService struct {
	DB        *sql.DB
	JWTSecret []byte
}

// NewTokenService creates a new token service
func NewTokenService(db *sql.DB, jwtSecret []byte) *TokenService {
	return &TokenService{
		DB:        db,
		JWTSecret: jwtSecret,
	}
}

// GenerateTokens creates a new access and refresh token pair
func (s *TokenService) GenerateTokens(userID int) (gin.H, error) {
	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, &models.Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	})
	accessTokenString, _ := accessToken.SignedString(s.JWTSecret)

	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return nil, err
	}
	refreshToken := hex.EncodeToString(bytes)

	if _, err := s.DB.Exec(
		`INSERT INTO refresh_tokens (user_id, token, expires_at) VALUES ($1, $2, $3)`,
		userID, refreshToken, time.Now().Add(365*24*time.Hour),
	); err != nil {
		return nil, err
	}

	return gin.H{"access_token": accessTokenString, "refresh_token": refreshToken}, nil
}

// ValidateRefreshToken checks if a refresh token is valid and returns the user ID
func (s *TokenService) ValidateRefreshToken(refreshToken string) (int, error) {
	var userID int
	err := s.DB.QueryRow(
		`SELECT user_id FROM refresh_tokens WHERE token = $1 AND expires_at > NOW()`,
		refreshToken,
	).Scan(&userID)

	if err != nil {
		return 0, err
	}

	return userID, nil
}

// InvalidateRefreshToken invalidates a refresh token
func (s *TokenService) InvalidateRefreshToken(refreshToken string) error {
	_, err := s.DB.Exec(`DELETE FROM refresh_tokens WHERE token = $1`, refreshToken)
	return err
}

// VerifyPassword checks if a password matches the hashed version
func VerifyPassword(hashedPassword, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password)) == nil
}

// HashPassword creates a bcrypt hash of a password
func HashPassword(password string) (string, error) {
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashedBytes), nil
}
