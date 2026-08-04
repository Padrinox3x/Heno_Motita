package dto

import (
	"time"

	"heno-motita-api/internal/models"
)

// CreateCrewRequest representa el cuerpo para crear una cuadrilla.
//
// startAt y endAt deben enviarse en formato RFC3339:
// 2026-08-10T06:00:00Z
type CreateCrewRequest struct {
	Name        string `json:"name" binding:"required,min=3,max=150"`
	Description string `json:"description" binding:"omitempty,max=500"`

	Zone        string `json:"zone" binding:"required,min=2,max=150"`
	Institution string `json:"institution" binding:"required,min=2,max=180"`

	ManagerID string `json:"managerId" binding:"required"`

	StartAt time.Time `json:"startAt" binding:"required"`
	EndAt   time.Time `json:"endAt" binding:"required"`

	StudentLimit int `json:"studentLimit" binding:"required,min=1,max=100"`
}

// UpdateCrewRequest representa una actualización completa.
//
// Al ser un PUT, deben enviarse todos los campos principales.
type UpdateCrewRequest struct {
	Name        string `json:"name" binding:"required,min=3,max=150"`
	Description string `json:"description" binding:"omitempty,max=500"`

	Zone        string `json:"zone" binding:"required,min=2,max=150"`
	Institution string `json:"institution" binding:"required,min=2,max=180"`

	ManagerID string `json:"managerId" binding:"required"`

	StartAt time.Time `json:"startAt" binding:"required"`
	EndAt   time.Time `json:"endAt" binding:"required"`

	StudentLimit int `json:"studentLimit" binding:"required,min=1,max=100"`
}

// UpdateCrewStatusRequest representa el cambio de estado.
type UpdateCrewStatusRequest struct {
	Status models.CrewStatus `json:"status" binding:"required"`
}

// CrewManagerResponse contiene información pública del encargado.
type CrewManagerResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Email       string `json:"email"`
	Phone       string `json:"phone"`
	Institution string `json:"institution"`
	Status      string `json:"status"`
}

// CrewResponse representa una cuadrilla sin exponer
// información sensible del encargado.
type CrewResponse struct {
	ID string `json:"id"`

	Name        string `json:"name"`
	Description string `json:"description"`

	Zone        string `json:"zone"`
	Institution string `json:"institution"`

	ManagerID string               `json:"managerId"`
	Manager   *CrewManagerResponse `json:"manager,omitempty"`

	StartAt time.Time `json:"startAt"`
	EndAt   time.Time `json:"endAt"`

	StudentLimit int               `json:"studentLimit"`
	Status       models.CrewStatus `json:"status"`

	CreatedBy string `json:"createdBy"`

	FinishedAt  *time.Time `json:"finishedAt,omitempty"`
	CancelledAt *time.Time `json:"cancelledAt,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// CrewListResponse representa una lista paginada.
//
// PaginationResponse ya fue declarado en manager_dto.go.
// No debes volver a declararlo.
type CrewListResponse struct {
	Crews      []CrewResponse     `json:"crews"`
	Pagination PaginationResponse `json:"pagination"`
}

// NewCrewResponse convierte los modelos internos en una respuesta.
func NewCrewResponse(
	crew models.Crew,
	manager *models.User,
) CrewResponse {
	response := CrewResponse{
		ID:           crew.ID.Hex(),
		Name:         crew.Name,
		Description:  crew.Description,
		Zone:         crew.Zone,
		Institution:  crew.Institution,
		ManagerID:    crew.ManagerID.Hex(),
		StartAt:      crew.StartAt,
		EndAt:        crew.EndAt,
		StudentLimit: crew.StudentLimit,
		Status:       crew.Status,
		CreatedBy:    crew.CreatedBy.Hex(),
		FinishedAt:   crew.FinishedAt,
		CancelledAt:  crew.CancelledAt,
		CreatedAt:    crew.CreatedAt,
		UpdatedAt:    crew.UpdatedAt,
	}

	if manager != nil {
		response.Manager = &CrewManagerResponse{
			ID:          manager.ID.Hex(),
			Name:        manager.Name,
			Email:       manager.Email,
			Phone:       manager.Phone,
			Institution: manager.Institution,
			Status:      string(manager.Status),
		}
	}

	return response
}
