package models

type AvatarRequest struct {
	Username  string `json:"username" binding:"required"`
	RoleID    int    `json:"role_id" binding:"required"`
	SquadID   int    `json:"squad_id" binding:"required"`
	CountryID int    `json:"country_id" binding:"required"`
	Status    string `json:"status" binding:"required"`
}

type AvatarResponse struct {
	Message string `json:"message"`
}

type UserSquad struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

type UserAvatarResponse struct {
	Username  string      `json:"username"`
	Roles     []string    `json:"roles"`
	Squads    []UserSquad `json:"squads"`
	Countries []string    `json:"countries"`
}
