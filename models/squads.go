package models

type Squad struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type CreateSquadRequest struct {
	Name string `json:"name" binding:"required"`
}

type SquadResponse struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}
