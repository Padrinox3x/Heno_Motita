package dto

import (
	"time"

	"heno-motita-api/internal/models"
)

// LoginRequest representa el JSON recibido en el inicio de sesión.
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// UserResponse evita devolver información sensible,
// como el hash de la contraseña.
type UserResponse struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Email     string            `json:"email"`
	Role      models.UserRole   `json:"role"`
	Status    models.UserStatus `json:"status"`
	CreatedAt time.Time         `json:"createdAt"`
	UpdatedAt time.Time         `json:"updatedAt"`
}

// LoginResponse representa el resultado del inicio de sesión.
type LoginResponse struct {
	Message     string       `json:"message"`
	AccessToken string       `json:"accessToken"`
	TokenType   string       `json:"tokenType"`
	ExpiresIn   int64        `json:"expiresIn"`
	ExpiresAt   time.Time    `json:"expiresAt"`
	User        UserResponse `json:"user"`
}

// NewUserResponse convierte el modelo interno a una respuesta segura.
func NewUserResponse(user models.User) UserResponse {
	return UserResponse{
		ID:        user.ID.Hex(),
		Name:      user.Name,
		Email:     user.Email,
		Role:      user.Role,
		Status:    user.Status,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}
}
