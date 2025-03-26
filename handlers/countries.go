package handlers

import (
	"database/sql"
	"net/http"

	"unicorn_app_backend/models"

	"github.com/gin-gonic/gin"
)

type CountryHandler struct {
	db *sql.DB
}

func NewCountryHandler(db *sql.DB) *CountryHandler {
	return &CountryHandler{db: db}
}

// CreateCountry handles the creation of a new country
func (h *CountryHandler) CreateCountry(c *gin.Context) {
	var req models.CreateCountryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var countryID int
	err := h.db.QueryRow(
		"INSERT INTO countries (name) VALUES ($1) RETURNING id",
		req.Name,
	).Scan(&countryID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create country"})
		return
	}

	c.JSON(http.StatusCreated, models.CountryResponse{
		ID:   countryID,
		Name: req.Name,
	})
}

// GetCountries handles retrieving all countries
func (h *CountryHandler) GetCountries(c *gin.Context) {
	rows, err := h.db.Query("SELECT id, name FROM countries")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch countries"})
		return
	}
	defer rows.Close()

	var countries []models.CountryResponse
	for rows.Next() {
		var country models.CountryResponse
		if err := rows.Scan(&country.ID, &country.Name); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to scan country"})
			return
		}
		countries = append(countries, country)
	}

	c.JSON(http.StatusOK, countries)
}
