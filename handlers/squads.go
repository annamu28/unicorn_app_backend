package handlers

import (
	"database/sql"
	"net/http"

	"unicorn_app_backend/models"

	"github.com/gin-gonic/gin"
)

type SquadHandler struct {
	db *sql.DB
}

func NewSquadHandler(db *sql.DB) *SquadHandler {
	return &SquadHandler{db: db}
}

// CreateSquad handles the creation of a new squad
func (h *SquadHandler) CreateSquad(c *gin.Context) {
	var req models.CreateSquadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var squadID int
	err := h.db.QueryRow(
		"INSERT INTO squads (name) VALUES ($1) RETURNING id",
		req.Name,
	).Scan(&squadID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create squad"})
		return
	}

	c.JSON(http.StatusCreated, models.SquadResponse{
		ID:   squadID,
		Name: req.Name,
	})
}

// GetSquads handles retrieving all squads
func (h *SquadHandler) GetSquads(c *gin.Context) {
	rows, err := h.db.Query("SELECT id, name FROM squads")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch squads"})
		return
	}
	defer rows.Close()

	var squads []models.SquadResponse
	for rows.Next() {
		var squad models.SquadResponse
		if err := rows.Scan(&squad.ID, &squad.Name); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to scan squad"})
			return
		}
		squads = append(squads, squad)
	}

	c.JSON(http.StatusOK, squads)
}
