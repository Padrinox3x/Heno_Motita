package dto

import (
	"time"

	"heno-motita-api/internal/models"
)

// StudentHistoryStudentResponse contiene la información
// pública del alumno consultado.
type StudentHistoryStudentResponse struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Email      string            `json:"email"`
	Enrollment string            `json:"enrollment"`
	Status     models.UserStatus `json:"status"`
}

// MembershipCrewHistoryResponse contiene la información
// básica de la cuadrilla asociada a una membresía.
type MembershipCrewHistoryResponse struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Zone        string            `json:"zone"`
	Institution string            `json:"institution"`
	Status      models.CrewStatus `json:"status"`
	StartAt     time.Time         `json:"startAt"`
	EndAt       time.Time         `json:"endAt"`
}

// MembershipHistoryResponse representa una participación
// histórica del alumno dentro de una cuadrilla.
type MembershipHistoryResponse struct {
	ID string `json:"id"`

	CrewID    string `json:"crewId"`
	StudentID string `json:"studentId"`

	Crew *MembershipCrewHistoryResponse `json:"crew,omitempty"`

	Status models.MembershipStatus `json:"status"`

	ValidFrom  time.Time `json:"validFrom"`
	ValidUntil time.Time `json:"validUntil"`

	ActivationCodeExpiresAt *time.Time `json:"activationCodeExpiresAt,omitempty"`
	ActivatedAt             *time.Time `json:"activatedAt,omitempty"`
	ArchivedAt              *time.Time `json:"archivedAt,omitempty"`

	CreatedByID string `json:"createdById"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// StudentMembershipHistoryResponse representa el historial
// completo de un alumno.
type StudentMembershipHistoryResponse struct {
	Student     StudentHistoryStudentResponse `json:"student"`
	Memberships []MembershipHistoryResponse   `json:"memberships"`
	Total       int                           `json:"total"`
}

// StudentHistoryListItemResponse representa un alumno
// dentro del listado histórico general.
type StudentHistoryListItemResponse struct {
	Student StudentHistoryStudentResponse `json:"student"`

	LatestMembership *MembershipHistoryResponse `json:"latestMembership,omitempty"`

	TotalMemberships int `json:"totalMemberships"`
}

// StudentHistoryListResponse representa el historial
// general paginado.
type StudentHistoryListResponse struct {
	Students   []StudentHistoryListItemResponse `json:"students"`
	Pagination PaginationResponse               `json:"pagination"`
}

// ReactivationCredentialResponse contiene el código
// temporal generado durante una reactivación.
//
// Este código solamente debe mostrarse una vez.
type ReactivationCredentialResponse struct {
	Email          string    `json:"email"`
	ActivationCode string    `json:"activationCode"`
	ExpiresAt      time.Time `json:"expiresAt"`
}

// ReactivateStudentResponse representa la respuesta
// al incorporar un alumno existente a otra cuadrilla.
type ReactivateStudentResponse struct {
	Student    StudentHistoryStudentResponse  `json:"student"`
	Membership MembershipHistoryResponse      `json:"membership"`
	Credential ReactivationCredentialResponse `json:"credential"`
}

// NewStudentHistoryStudentResponse convierte un usuario
// en una respuesta pública.
func NewStudentHistoryStudentResponse(
	student models.User,
) StudentHistoryStudentResponse {
	return StudentHistoryStudentResponse{
		ID:         student.ID.Hex(),
		Name:       student.Name,
		Email:      student.Email,
		Enrollment: student.Enrollment,
		Status:     student.Status,
	}
}

// NewMembershipHistoryResponse convierte una membresía
// en una respuesta pública.
func NewMembershipHistoryResponse(
	membership models.CrewMembership,
	crew *models.Crew,
) MembershipHistoryResponse {
	response := MembershipHistoryResponse{
		ID:                      membership.ID.Hex(),
		CrewID:                  membership.CrewID.Hex(),
		StudentID:               membership.StudentID.Hex(),
		Status:                  membership.Status,
		ValidFrom:               membership.ValidFrom,
		ValidUntil:              membership.ValidUntil,
		ActivationCodeExpiresAt: membership.ActivationCodeExpiresAt,
		ActivatedAt:             membership.ActivatedAt,
		ArchivedAt:              membership.ArchivedAt,
		CreatedByID:             membership.CreatedBy.Hex(),
		CreatedAt:               membership.CreatedAt,
		UpdatedAt:               membership.UpdatedAt,
	}

	if crew != nil {
		response.Crew = &MembershipCrewHistoryResponse{
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
