package models

type CreateAvatarRequest struct {
	Username   string      `json:"username"`
	SquadRoles []SquadRole `json:"squad_roles"`
	CountryID  int         `json:"country_id"`
}

type SquadRequest struct {
	ID     int      `json:"id"`
	Status string   `json:"status"`
	Roles  []string `json:"roles"`
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
	ID     int      `json:"id"`
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

type VerificationRequest struct {
	UserID  int    `json:"user_id" binding:"required"`
	SquadID int    `json:"squad_id" binding:"required"`
	Status  string `json:"status" binding:"required,oneof=Pending Approved Rejected"`
}
