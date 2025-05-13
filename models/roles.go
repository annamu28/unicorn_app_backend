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

type AssignGlobalRoleRequest struct {
	UserID int `json:"user_id" binding:"required"`
	RoleID int `json:"role_id" binding:"required"`
}

type GlobalRoleResponse struct {
	ID        int    `json:"id"`
	UserID    int    `json:"user_id"`
	RoleID    int    `json:"role_id"`
	RoleName  string `json:"role_name"`
	CreatedAt string `json:"created_at"`
}
