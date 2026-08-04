package dto

import (
	"time"

	"heno-motita-api/internal/models"
)

// CreateStudentInput representa un alumno dentro
// del registro masivo.
type CreateStudentInput struct {
	Name       string `json:"name" binding:"required,min=3,max=120"`
	Email      string `json:"email" binding:"required,email,max=160"`
	Enrollment string `json:"enrollment" binding:"required,min=3,max=30"`
}

// BatchCreateStudentsRequest contiene los alumnos
// que se agregarán a una cuadrilla.
type BatchCreateStudentsRequest struct {
	Students []CreateStudentInput `json:"students" binding:"required,min=1,max=100,dive"`
}

// UpdateStudentRequest representa la actualización
// de los datos generales de un alumno.
type UpdateStudentRequest struct {
	Name       string `json:"name" binding:"required,min=3,max=120"`
	Email      string `json:"email" binding:"required,email,max=160"`
	Enrollment string `json:"enrollment" binding:"required,min=3,max=30"`
}

// UpdateStudentStatusRequest permite cambiar el estado
// general de la cuenta.
type UpdateStudentStatusRequest struct {
	Status models.UserStatus `json:"status" binding:"required"`
}

// ActivateStudentRequest permite establecer la contraseña
// inicial utilizando un código temporal.
type ActivateStudentRequest struct {
	Email          string `json:"email" binding:"required,email"`
	ActivationCode string `json:"activationCode" binding:"required"`
	Password       string `json:"password" binding:"required,min=8,max=72"`
}

// ActivationCredential contiene el código que se mostrará
// solamente al momento de generarlo.
type ActivationCredential struct {
	StudentID      string    `json:"studentId"`
	Name           string    `json:"name"`
	Email          string    `json:"email"`
	Enrollment     string    `json:"enrollment"`
	ActivationCode string    `json:"activationCode"`
	ExpiresAt      time.Time `json:"expiresAt"`
}

type BatchCreateStudentsResponse struct {
	Message     string                 `json:"message"`
	CrewID      string                 `json:"crewId"`
	Registered  int                    `json:"registered"`
	Credentials []ActivationCredential `json:"credentials"`
}

type StudentCrewResponse struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Zone        string            `json:"zone"`
	Institution string            `json:"institution"`
	Status      models.CrewStatus `json:"status"`
	StartAt     time.Time         `json:"startAt"`
	EndAt       time.Time         `json:"endAt"`
}

type StudentMembershipResponse struct {
	ID string `json:"id"`

	Status models.MembershipStatus `json:"status"`

	ValidFrom  time.Time `json:"validFrom"`
	ValidUntil time.Time `json:"validUntil"`

	ActivationCodeExpiresAt *time.Time `json:"activationCodeExpiresAt,omitempty"`
	ActivatedAt             *time.Time `json:"activatedAt,omitempty"`
	ArchivedAt              *time.Time `json:"archivedAt,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type StudentResponse struct {
	ID string `json:"id"`

	Name       string `json:"name"`
	Email      string `json:"email"`
	Enrollment string `json:"enrollment"`

	Role   models.UserRole   `json:"role"`
	Status models.UserStatus `json:"status"`

	Membership *StudentMembershipResponse `json:"membership,omitempty"`
	Crew       *StudentCrewResponse       `json:"crew,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type StudentListResponse struct {
	Students   []StudentResponse  `json:"students"`
	Pagination PaginationResponse `json:"pagination"`
}

// NewStudentResponse genera una respuesta segura.
func NewStudentResponse(
	student models.User,
	membership *models.CrewMembership,
	crew *models.Crew,
) StudentResponse {
	response := StudentResponse{
		ID:         student.ID.Hex(),
		Name:       student.Name,
		Email:      student.Email,
		Enrollment: student.Enrollment,
		Role:       student.Role,
		Status:     student.Status,
		CreatedAt:  student.CreatedAt,
		UpdatedAt:  student.UpdatedAt,
	}

	if membership != nil {
		response.Membership = &StudentMembershipResponse{
			ID:                      membership.ID.Hex(),
			Status:                  membership.Status,
			ValidFrom:               membership.ValidFrom,
			ValidUntil:              membership.ValidUntil,
			ActivationCodeExpiresAt: membership.ActivationCodeExpiresAt,
			ActivatedAt:             membership.ActivatedAt,
			ArchivedAt:              membership.ArchivedAt,
			CreatedAt:               membership.CreatedAt,
			UpdatedAt:               membership.UpdatedAt,
		}
	}

	if crew != nil {
		response.Crew = &StudentCrewResponse{
			ID:          crew.ID.Hex(),
			Name:        crew.Name,
			Zone:        crew.Zone,
			Institution: crew.Institution,
			Status:      crew.Status,
			StartAt:     crew.StartAt,
			EndAt:       crew.EndAt,
		}
	}

	return response
}
