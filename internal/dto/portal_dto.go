package dto

import "time"

// PortalUserResponse contiene la información pública
// de un usuario dentro de los paneles.
type PortalUserResponse struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Email      string `json:"email"`
	Enrollment string `json:"enrollment,omitempty"`
	Role       string `json:"role"`
	Status     string `json:"status"`
}

// PortalCrewResponse contiene los datos principales
// de una cuadrilla.
type PortalCrewResponse struct {
	ID string `json:"id"`

	Name        string `json:"name"`
	Zone        string `json:"zone"`
	Institution string `json:"institution"`

	ManagerID string `json:"managerId"`

	Status string `json:"status"`

	StartAt time.Time `json:"startAt"`
	EndAt   time.Time `json:"endAt"`

	StudentLimit int `json:"studentLimit"`
}

// PortalMembershipResponse representa la membresía
// vigente de un alumno.
type PortalMembershipResponse struct {
	ID string `json:"id"`

	CrewID    string `json:"crewId"`
	StudentID string `json:"studentId"`

	Status string `json:"status"`

	ValidFrom  time.Time `json:"validFrom"`
	ValidUntil time.Time `json:"validUntil"`

	ActivatedAt *time.Time `json:"activatedAt,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// PortalCrewStats contiene las estadísticas básicas
// de una cuadrilla.
type PortalCrewStats struct {
	Students     int64 `json:"students"`
	Trees        int64 `json:"trees"`
	Observations int64 `json:"observations"`
	Images       int64 `json:"images"`
}

// ManagerCurrentCrewResponse contiene una cuadrilla
// asignada al encargado y sus estadísticas.
type ManagerCurrentCrewResponse struct {
	Crew  PortalCrewResponse `json:"crew"`
	Stats PortalCrewStats    `json:"stats"`
}

// ManagerDashboardSummary contiene los totales
// generales del encargado.
type ManagerDashboardSummary struct {
	AssignedCrews  int64 `json:"assignedCrews"`
	PendingCrews   int64 `json:"pendingCrews"`
	ActiveCrews    int64 `json:"activeCrews"`
	FinishedCrews  int64 `json:"finishedCrews"`
	CancelledCrews int64 `json:"cancelledCrews"`

	CurrentStudents     int64 `json:"currentStudents"`
	CurrentTrees        int64 `json:"currentTrees"`
	CurrentObservations int64 `json:"currentObservations"`
	CurrentImages       int64 `json:"currentImages"`
}

// ManagerDashboardResponse representa la información
// principal del panel del encargado.
type ManagerDashboardResponse struct {
	Manager PortalUserResponse `json:"manager"`

	Summary ManagerDashboardSummary `json:"summary"`

	CurrentCrews []ManagerCurrentCrewResponse `json:"currentCrews"`
}

// ManagerCurrentCrewsResponse contiene la lista paginada
// de cuadrillas vigentes del encargado.
type ManagerCurrentCrewsResponse struct {
	Crews      []ManagerCurrentCrewResponse `json:"crews"`
	Pagination PaginationResponse           `json:"pagination"`
}

// StudentProfileResponse representa el perfil del alumno
// y su asignación vigente.
type StudentProfileResponse struct {
	Student PortalUserResponse `json:"student"`

	CurrentMembership *PortalMembershipResponse `json:"currentMembership,omitempty"`
	CurrentCrew       *PortalCrewResponse       `json:"currentCrew,omitempty"`
}

// StudentCurrentCrewResponse contiene la cuadrilla vigente
// del alumno, el encargado y estadísticas generales.
type StudentCurrentCrewResponse struct {
	Crew       PortalCrewResponse       `json:"crew"`
	Membership PortalMembershipResponse `json:"membership"`
	Manager    *PortalUserResponse      `json:"manager,omitempty"`
	Stats      PortalCrewStats          `json:"stats"`
}

// StudentTreeResponse representa un árbol visible
// desde el panel del alumno.
type StudentTreeResponse struct {
	ID string `json:"id"`

	CrewID string `json:"crewId"`

	Code           string `json:"code"`
	CommonName     string `json:"commonName"`
	ScientificName string `json:"scientificName"`

	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`

	LocationDescription string `json:"locationDescription,omitempty"`

	Status string `json:"status"`

	RegisteredBy   string `json:"registeredBy"`
	RegisteredByMe bool   `json:"registeredByMe"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// StudentTreeListResponse representa la lista paginada
// de árboles de la cuadrilla vigente.
type StudentTreeListResponse struct {
	Trees      []StudentTreeResponse `json:"trees"`
	Pagination PaginationResponse    `json:"pagination"`
}

// PortalTreeSummaryResponse contiene información breve
// del árbol relacionado con una observación.
type PortalTreeSummaryResponse struct {
	ID string `json:"id"`

	Code           string `json:"code"`
	CommonName     string `json:"commonName"`
	ScientificName string `json:"scientificName"`
}

// StudentObservationResponse representa una observación
// realizada por el alumno autenticado.
type StudentObservationResponse struct {
	ID string `json:"id"`

	TreeID string `json:"treeId"`
	CrewID string `json:"crewId"`

	Tree *PortalTreeSummaryResponse `json:"tree,omitempty"`

	Method string `json:"method"`

	LowerThirdScore  int `json:"lowerThirdScore"`
	MiddleThirdScore int `json:"middleThirdScore"`
	UpperThirdScore  int `json:"upperThirdScore"`
	TotalScore       int `json:"totalScore"`

	Notes string `json:"notes,omitempty"`

	ObservationDate time.Time `json:"observationDate"`

	Latitude  *float64 `json:"latitude,omitempty"`
	Longitude *float64 `json:"longitude,omitempty"`

	Status string `json:"status"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// StudentObservationListResponse representa las
// observaciones del alumno.
type StudentObservationListResponse struct {
	Observations []StudentObservationResponse `json:"observations"`
	Pagination   PaginationResponse           `json:"pagination"`
}
