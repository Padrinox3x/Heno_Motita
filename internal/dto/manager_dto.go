package dto

import (
	"time"

	"heno-motita-api/internal/models"
)

// CreateManagerRequest representa el JSON utilizado
// para registrar un encargado.
type CreateManagerRequest struct {
	Name        string `json:"name" binding:"required,min=3,max=120"`
	Email       string `json:"email" binding:"required,email,max=160"`
	Password    string `json:"password" binding:"required,min=8,max=72"`
	Phone       string `json:"phone" binding:"omitempty,max=20"`
	Institution string `json:"institution" binding:"required,min=2,max=160"`
}

// UpdateManagerRequest representa la edición completa
// de los datos principales del encargado.
//
// La contraseña es opcional. Si no se envía,
// se conserva la contraseña actual.
type UpdateManagerRequest struct {
	Name        string `json:"name" binding:"required,min=3,max=120"`
	Email       string `json:"email" binding:"required,email,max=160"`
	Password    string `json:"password" binding:"omitempty,min=8,max=72"`
	Phone       string `json:"phone" binding:"omitempty,max=20"`
	Institution string `json:"institution" binding:"required,min=2,max=160"`
}

// UpdateManagerStatusRequest permite activar,
// desactivar o bloquear a un encargado.
type UpdateManagerStatusRequest struct {
	Status models.UserStatus `json:"status" binding:"required"`
}

// ManagerResponse evita devolver passwordHash.
type ManagerResponse struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Email       string            `json:"email"`
	Phone       string            `json:"phone"`
	Institution string            `json:"institution"`
	Role        models.UserRole   `json:"role"`
	Status      models.UserStatus `json:"status"`
	CreatedAt   time.Time         `json:"createdAt"`
	UpdatedAt   time.Time         `json:"updatedAt"`
}

type PaginationResponse struct {
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	Total      int64 `json:"total"`
	TotalPages int64 `json:"totalPages"`
}

type ManagerListResponse struct {
	Managers   []ManagerResponse  `json:"managers"`
	Pagination PaginationResponse `json:"pagination"`
}

// NewManagerResponse convierte el modelo de MongoDB
// en una respuesta segura.
func NewManagerResponse(
	manager models.User,
) ManagerResponse {
	return ManagerResponse{
		ID:          manager.ID.Hex(),
		Name:        manager.Name,
		Email:       manager.Email,
		Phone:       manager.Phone,
		Institution: manager.Institution,
		Role:        manager.Role,
		Status:      manager.Status,
		CreatedAt:   manager.CreatedAt,
		UpdatedAt:   manager.UpdatedAt,
	}
}
