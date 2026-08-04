package dto

import (
	"time"

	"heno-motita-api/internal/models"
)

// CreateObservationRequest representa el cuerpo para
// registrar una evaluación Hawksworth.
//
// Se utilizan punteros en las puntuaciones porque cero
// es un valor válido.
type CreateObservationRequest struct {
	LowerThirdScore  *int `json:"lowerThirdScore" binding:"required"`
	MiddleThirdScore *int `json:"middleThirdScore" binding:"required"`
	UpperThirdScore  *int `json:"upperThirdScore" binding:"required"`

	Notes string `json:"notes" binding:"omitempty,max=1000"`

	ObservationDate time.Time `json:"observationDate" binding:"required"`

	Latitude  *float64 `json:"latitude"`
	Longitude *float64 `json:"longitude"`
}

// UpdateObservationRequest representa una edición completa.
type UpdateObservationRequest struct {
	LowerThirdScore  *int `json:"lowerThirdScore" binding:"required"`
	MiddleThirdScore *int `json:"middleThirdScore" binding:"required"`
	UpperThirdScore  *int `json:"upperThirdScore" binding:"required"`

	Notes string `json:"notes" binding:"omitempty,max=1000"`

	ObservationDate time.Time `json:"observationDate" binding:"required"`

	Latitude  *float64 `json:"latitude"`
	Longitude *float64 `json:"longitude"`
}

// UpdateObservationStatusRequest permite archivar
// una observación.
type UpdateObservationStatusRequest struct {
	Status models.ObservationStatus `json:"status" binding:"required"`
}

// ObservationTreeResponse contiene la información básica
// del árbol evaluado.
type ObservationTreeResponse struct {
	ID             string `json:"id"`
	Code           string `json:"code"`
	CommonName     string `json:"commonName"`
	ScientificName string `json:"scientificName,omitempty"`
}

// ObservationUserResponse contiene información pública
// del usuario que realizó la evaluación.
type ObservationUserResponse struct {
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Email string          `json:"email"`
	Role  models.UserRole `json:"role"`
}

// HawksworthResponse contiene el desglose de la evaluación.
type HawksworthResponse struct {
	LowerThirdScore  int `json:"lowerThirdScore"`
	MiddleThirdScore int `json:"middleThirdScore"`
	UpperThirdScore  int `json:"upperThirdScore"`

	TotalScore    int `json:"totalScore"`
	MaximumScore  int `json:"maximumScore"`
	MinimumScore  int `json:"minimumScore"`
	MaximumByPart int `json:"maximumByThird"`
}

// ObservationResponse representa la respuesta pública
// de una observación.
type ObservationResponse struct {
	ID string `json:"id"`

	TreeID string `json:"treeId"`
	CrewID string `json:"crewId"`

	ObserverID string                   `json:"observerId"`
	Observer   *ObservationUserResponse `json:"observer,omitempty"`

	Tree *ObservationTreeResponse `json:"tree,omitempty"`

	Method models.AssessmentMethod `json:"method"`

	Hawksworth HawksworthResponse `json:"hawksworth"`

	Notes string `json:"notes,omitempty"`

	ObservationDate time.Time `json:"observationDate"`

	Latitude  *float64 `json:"latitude,omitempty"`
	Longitude *float64 `json:"longitude,omitempty"`

	Status models.ObservationStatus `json:"status"`

	ArchivedAt *time.Time `json:"archivedAt,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// ObservationListResponse representa una lista paginada.
type ObservationListResponse struct {
	Observations []ObservationResponse `json:"observations"`
	Pagination   PaginationResponse    `json:"pagination"`
}

// NewObservationResponse convierte los modelos internos
// en una respuesta segura.
func NewObservationResponse(
	observation models.Observation,
	tree *models.Tree,
	observer *models.User,
) ObservationResponse {
	response := ObservationResponse{
		ID:              observation.ID.Hex(),
		TreeID:          observation.TreeID.Hex(),
		CrewID:          observation.CrewID.Hex(),
		ObserverID:      observation.ObserverID.Hex(),
		Method:          observation.Method,
		Notes:           observation.Notes,
		ObservationDate: observation.ObservationDate,
		Latitude:        observation.Latitude,
		Longitude:       observation.Longitude,
		Status:          observation.Status,
		ArchivedAt:      observation.ArchivedAt,
		CreatedAt:       observation.CreatedAt,
		UpdatedAt:       observation.UpdatedAt,

		Hawksworth: HawksworthResponse{
			LowerThirdScore:  observation.LowerThirdScore,
			MiddleThirdScore: observation.MiddleThirdScore,
			UpperThirdScore:  observation.UpperThirdScore,
			TotalScore:       observation.TotalScore,
			MinimumScore:     0,
			MaximumScore:     6,
			MaximumByPart:    2,
		},
	}

	if tree != nil {
		response.Tree = &ObservationTreeResponse{
			ID:             tree.ID.Hex(),
			Code:           tree.Code,
			CommonName:     tree.CommonName,
			ScientificName: tree.ScientificName,
		}
	}

	if observer != nil {
		response.Observer = &ObservationUserResponse{
			ID:    observer.ID.Hex(),
			Name:  observer.Name,
			Email: observer.Email,
			Role:  observer.Role,
		}
	}

	return response
}
