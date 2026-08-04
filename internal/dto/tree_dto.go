package dto

import (
	"time"

	"heno-motita-api/internal/models"
)

// CreateTreeRequest representa el cuerpo utilizado
// para registrar un árbol.
type CreateTreeRequest struct {
	Code string `json:"code" binding:"required,min=3,max=40"`

	CommonName     string `json:"commonName" binding:"required,min=2,max=120"`
	ScientificName string `json:"scientificName" binding:"omitempty,max=160"`

	Latitude  *float64 `json:"latitude" binding:"required"`
	Longitude *float64 `json:"longitude" binding:"required"`

	LocationDescription string `json:"locationDescription" binding:"omitempty,max=500"`
}

// UpdateTreeRequest representa la edición completa
// de un árbol.
type UpdateTreeRequest struct {
	Code string `json:"code" binding:"required,min=3,max=40"`

	CommonName     string `json:"commonName" binding:"required,min=2,max=120"`
	ScientificName string `json:"scientificName" binding:"omitempty,max=160"`

	Latitude  *float64 `json:"latitude" binding:"required"`
	Longitude *float64 `json:"longitude" binding:"required"`

	LocationDescription string `json:"locationDescription" binding:"omitempty,max=500"`
}

// UpdateTreeStatusRequest permite cambiar el estado
// de un árbol.
type UpdateTreeStatusRequest struct {
	Status models.TreeStatus `json:"status" binding:"required"`
}

// TreeRegisteredByResponse contiene información pública
// del usuario que registró el árbol.
type TreeRegisteredByResponse struct {
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Email string          `json:"email"`
	Role  models.UserRole `json:"role"`
}

// TreeResponse representa la respuesta pública
// del módulo de árboles.
type TreeResponse struct {
	ID string `json:"id"`

	CrewID string `json:"crewId"`

	Code string `json:"code"`

	CommonName     string `json:"commonName"`
	ScientificName string `json:"scientificName,omitempty"`

	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`

	LocationDescription string `json:"locationDescription,omitempty"`

	Status models.TreeStatus `json:"status"`

	RegisteredByID string                    `json:"registeredById"`
	RegisteredBy   *TreeRegisteredByResponse `json:"registeredBy,omitempty"`

	ArchivedAt *time.Time `json:"archivedAt,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// TreeListResponse representa una lista paginada.
type TreeListResponse struct {
	Trees      []TreeResponse     `json:"trees"`
	Pagination PaginationResponse `json:"pagination"`
}

// NewTreeResponse convierte el modelo interno
// en una respuesta segura.
func NewTreeResponse(
	tree models.Tree,
	registeredBy *models.User,
) TreeResponse {
	response := TreeResponse{
		ID:                  tree.ID.Hex(),
		CrewID:              tree.CrewID.Hex(),
		Code:                tree.Code,
		CommonName:          tree.CommonName,
		ScientificName:      tree.ScientificName,
		Latitude:            tree.Latitude,
		Longitude:           tree.Longitude,
		LocationDescription: tree.LocationDescription,
		Status:              tree.Status,
		RegisteredByID:      tree.RegisteredBy.Hex(),
		ArchivedAt:          tree.ArchivedAt,
		CreatedAt:           tree.CreatedAt,
		UpdatedAt:           tree.UpdatedAt,
	}

	if registeredBy != nil {
		response.RegisteredBy = &TreeRegisteredByResponse{
			ID:    registeredBy.ID.Hex(),
			Name:  registeredBy.Name,
			Email: registeredBy.Email,
			Role:  registeredBy.Role,
		}
	}

	return response
}
