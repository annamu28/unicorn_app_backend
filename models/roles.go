package models

type Role struct {
	ID   int    `json:"id"`
	Role string `json:"role"`
}

type CreateRoleRequest struct {
	Role string `json:"role" binding:"required"`
}

type RoleResponse struct {
	ID   int    `json:"id"`
	Role string `json:"role"`
}
