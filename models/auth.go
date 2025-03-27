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
	Email    string `json:"email"`
	Password string `json:"password"`
}

type AuthResponse struct {
	Token string `json:"token"`
}

type Claims struct {
	UserID int `json:"user_id"`
	jwt.RegisteredClaims
}

type LoginResponse struct {
	Token        string      `json:"token"`
	RefreshToken string      `json:"refresh_token"`
	UserInfo     UserProfile `json:"user_info"`
}

type UserProfile struct {
	Username  string      `json:"username"`
	Roles     []string    `json:"roles"`
	Squads    []UserSquad `json:"squads"`
	Countries []string    `json:"countries"`
}