package models

import (
	"github.com/golang-jwt/jwt/v5"
)

type RegisterRequest struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
	Password  string `json:"password"`
	Birthday  string `json:"birthday"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type AuthResponse struct {
	Token string `json:"token"`
}

type Claims struct {
	UserID int `json:"user_id"`
	jwt.RegisteredClaims
}

type LoginResponse struct {
	AccessToken  string      `json:"access_token"`
	RefreshToken string      `json:"refresh_token"`
	UserID       int         `json:"user_id"`
	FirstName    string      `json:"first_name"`
	LastName     string      `json:"last_name"`
	Email        string      `json:"email"`
	Profile      UserProfile `json:"profile"`
}

type UserProfile struct {
	Username  string      `json:"username"`
	Roles     []string    `json:"roles"`
	Squads    []UserSquad `json:"squads"`
	Countries []string    `json:"countries"`
}

type User struct {
	ID           int    `json:"id"`
	Email        string `json:"email"`
	PasswordHash string `json:"-"` // "-" means this field won't be included in JSON
}
