package handlers

import (
	"database/sql"
	"net/http"

	"unicorn_app_backend/models"

	"github.com/gin-gonic/gin"
)

type RoleHandler struct {
	db *sql.DB
}

func NewRoleHandler(db *sql.DB) *RoleHandler {
	return &RoleHandler{db: db}
}

// CreateRole handles the creation of a new role
func (h *RoleHandler) CreateRole(c *gin.Context) {
	var req models.CreateRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var roleID int
	err := h.db.QueryRow(
		"INSERT INTO roles (role) VALUES ($1) RETURNING id",
		req.Role,
	).Scan(&roleID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create role"})
		return
	}

	c.JSON(http.StatusCreated, models.RoleResponse{
		ID:   roleID,
		Role: req.Role,
	})
}

// GetRoles handles retrieving all roles
func (h *RoleHandler) GetRoles(c *gin.Context) {
	rows, err := h.db.Query("SELECT id, role FROM roles")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch roles"})
		return
	}
	defer rows.Close()

	var roles []models.RoleResponse
	for rows.Next() {
		var role models.RoleResponse
		if err := rows.Scan(&role.ID, &role.Role); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to scan role"})
			return
		}
		roles = append(roles, role)
	}

	c.JSON(http.StatusOK, roles)
}
