package handlers

import (
	"database/sql"
	"net/http"
	"time"
	"unicorn_app_backend/models"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type AuthHandler struct {
	db     *sql.DB
	secret []byte
}

func NewAuthHandler(db *sql.DB, secret []byte) *AuthHandler {
	return &AuthHandler{
		db:     db,
		secret: secret,
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

// LoginRequest matches the Flutter frontend's login request
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// LoginResponse matches what the Flutter frontend expects
type LoginResponse struct {
	Token  string `json:"token"`
	Result string `json:"result"` // "success" or "failure"
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, LoginResponse{
			Result: "failure",
		})
		return
	}

	// Get user from database
	var (
		userID       int
		passwordHash string
	)
	err := h.db.QueryRow("SELECT id, password_hash FROM users WHERE email = $1",
		req.Email).Scan(&userID, &passwordHash)

	if err != nil {
		c.JSON(http.StatusUnauthorized, LoginResponse{
			Result: "failure",
		})
		return
	}

	// Check password
	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, LoginResponse{
			Result: "failure",
		})
		return
	}

	// Generate JWT token
	token := h.generateToken(userID)

	// Return success response
	c.JSON(http.StatusOK, LoginResponse{
		Token:  token,
		Result: "success",
	})
}

func (h *AuthHandler) generateToken(userID int) string {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": userID,
		"exp":     time.Now().Add(time.Hour * 24).Unix(),
	})

	tokenString, _ := token.SignedString(h.secret)
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
