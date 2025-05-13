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
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required,min=8"`
		Username string `json:"username" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

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
	err = h.db.QueryRow(
		`INSERT INTO users (email, password, username) VALUES ($1, $2, $3) RETURNING id`,
		req.Email, hashedPassword, req.Username,
	).Scan(&userID)

	if err != nil {
		log.Printf("Error creating user: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	tokens, err := h.tokenService.GenerateTokens(userID)
	if err != nil {
		log.Printf("Error generating tokens: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate tokens"})
		return
	}

	c.JSON(http.StatusCreated, tokens)
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

	var userID int
	var hashedPassword string
	err := h.db.QueryRow(`SELECT id, password FROM users WHERE email = $1`, req.Email).Scan(&userID, &hashedPassword)

	if err == sql.ErrNoRows || !middleware.VerifyPassword(hashedPassword, req.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	} else if err != nil {
		log.Printf("Error querying user: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify credentials"})
		return
	}

	tokens, err := h.tokenService.GenerateTokens(userID)
	if err != nil {
		log.Printf("Error generating tokens: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate tokens"})
		return
	}

	c.JSON(http.StatusOK, tokens)
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
	token := c.GetString("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No token provided"})
		return
	}

	if err := h.tokenService.InvalidateRefreshToken(token); err != nil {
		log.Printf("Error invalidating refresh token: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to logout"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Successfully logged out"})
}
