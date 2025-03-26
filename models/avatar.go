package models

type CreateAvatarRequest struct {
	Username   string      `json:"username" binding:"required"`
	SquadRoles []SquadRole `json:"squad_roles" binding:"required"`
	CountryID  int         `json:"country_id" binding:"required"`
}

type SquadRole struct {
	SquadID int    `json:"squad_id" binding:"required"`
	RoleID  int    `json:"role_id" binding:"required"`
	Status  string `json:"status" binding:"required"`
}

type AvatarResponse struct {
	Message string `json:"message"`
}

type UserSquad struct {
	Name   string   `json:"name"`
	Status string   `json:"status"`
	Roles  []string `json:"roles"`
}

type UserAvatarResponse struct {
	Username  string      `json:"username"`
	Roles     []string    `json:"roles"`
	Squads    []UserSquad `json:"squads"`
	Countries []string    `json:"countries"`
}
