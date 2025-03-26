package models

type Country struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type CreateCountryRequest struct {
	Name string `json:"name" binding:"required"`
}

type CountryResponse struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}
