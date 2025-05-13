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

// AssignGlobalRole handles assigning a global role to a user
func (h *RoleHandler) AssignGlobalRole(c *gin.Context) {
	var req models.AssignGlobalRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Start a transaction
	tx, err := h.db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction"})
		return
	}
	defer tx.Rollback()

	// Check if user exists
	var userExists bool
	err = tx.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)", req.UserID).Scan(&userExists)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check user existence"})
		return
	}
	if !userExists {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Check if role exists
	var roleExists bool
	err = tx.QueryRow("SELECT EXISTS(SELECT 1 FROM roles WHERE id = $1)", req.RoleID).Scan(&roleExists)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check role existence"})
		return
	}
	if !roleExists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Role not found"})
		return
	}

	// Insert the global role assignment
	var response models.GlobalRoleResponse
	err = tx.QueryRow(`
		INSERT INTO user_roles (user_id, role_id, created_at)
		VALUES ($1, $2, CURRENT_DATE)
		RETURNING id, user_id, role_id, created_at`,
		req.UserID, req.RoleID,
	).Scan(&response.ID, &response.UserID, &response.RoleID, &response.CreatedAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to assign role"})
		return
	}

	// Get the role name
	err = tx.QueryRow("SELECT role FROM roles WHERE id = $1", req.RoleID).Scan(&response.RoleName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get role name"})
		return
	}

	// Commit the transaction
	if err = tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
		return
	}

	c.JSON(http.StatusCreated, response)
}
