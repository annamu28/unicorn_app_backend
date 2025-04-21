package models

type CreateChatboardRequest struct {
	Title       string          `json:"title" binding:"required"`
	Description string          `json:"description" binding:"required"`
	Access      ChatboardAccess `json:"access" binding:"required"`
}

type ChatboardAccess struct {
	SquadIDs   []int `json:"squad_ids,omitempty"`   // Squads that can access
	RoleIDs    []int `json:"role_ids,omitempty"`    // Roles that can access
	CountryIDs []int `json:"country_ids,omitempty"` // Countries that can access
}

type ChatboardResponse struct {
	ID          int                 `json:"id"`
	Title       string              `json:"title"`
	Description string              `json:"description"`
	CreatedAt   string              `json:"created_at"`
	Access      ChatboardAccessInfo `json:"access"`
}

type ChatboardAccessInfo struct {
	Squads    []string `json:"squads"`
	Roles     []string `json:"roles"`
	Countries []string `json:"countries"`
}

type PendingUserResponse struct {
	UserID    int    `json:"user_id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
	SquadID   int    `json:"squad_id"`
	SquadName string `json:"squad_name"`
	Status    string `json:"status"`
}
