package handlers

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"heno-motita-api/internal/database"
	"heno-motita-api/internal/dto"
	"heno-motita-api/internal/middleware"
	"heno-motita-api/internal/models"
	"heno-motita-api/internal/security"
	"heno-motita-api/internal/services"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// StudentHandler contiene las colecciones necesarias
// para alumnos y membresías.
type StudentHandler struct {
	users        *mongo.Collection
	crews        *mongo.Collection
	memberships  *mongo.Collection
	emailService *services.EmailService
}

func NewStudentHandler(
	mongodb *database.MongoDB,
	emailService *services.EmailService,
) *StudentHandler {
	return &StudentHandler{
		users:        mongodb.Collection("users"),
		crews:        mongodb.Collection("crews"),
		memberships:  mongodb.Collection("crew_memberships"),
		emailService: emailService,
	}
}

// BatchCreate procesa:
//
// POST /api/v1/crews/:id/students/batch
func (handler *StudentHandler) BatchCreate(
	c *gin.Context,
) {
	crew, ok := middleware.GetCurrentCrew(c)
	if !ok {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "No se pudo recuperar la cuadrilla",
			},
		)
		return
	}

	var request dto.BatchCreateStudentsRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(
			http.StatusBadRequest,
			gin.H{
				"status":  "error",
				"message": "La lista de alumnos no es válida",
				"details": err.Error(),
			},
		)
		return
	}

	now := time.Now().UTC()

	if crew.Status == models.CrewStatusFinished ||
		crew.Status == models.CrewStatusCancelled ||
		!crew.EndAt.After(now) {
		c.JSON(
			http.StatusConflict,
			gin.H{
				"status":  "error",
				"message": "No se pueden agregar alumnos a una cuadrilla terminada o cancelada",
			},
		)
		return
	}

	currentUser, ok := middleware.GetCurrentUser(c)
	if !ok {
		c.JSON(
			http.StatusUnauthorized,
			gin.H{
				"status":  "error",
				"message": "No se encontró una sesión válida",
			},
		)
		return
	}

	ctx, cancel := context.WithTimeout(
		c.Request.Context(),
		30*time.Second,
	)
	defer cancel()

	currentMembers, err := handler.memberships.CountDocuments(
		ctx,
		bson.M{
			"crewId": crew.ID,
			"status": bson.M{
				"$in": bson.A{
					models.MembershipStatusPending,
					models.MembershipStatusActive,
					models.MembershipStatusInactive,
				},
			},
		},
	)
	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "No fue posible contar los alumnos de la cuadrilla",
			},
		)
		return
	}

	if currentMembers+int64(len(request.Students)) >
		int64(crew.StudentLimit) {
		c.JSON(
			http.StatusConflict,
			gin.H{
				"status":            "error",
				"message":           "La cantidad de alumnos supera el límite de la cuadrilla",
				"studentLimit":      crew.StudentLimit,
				"currentStudents":   currentMembers,
				"requestedStudents": len(request.Students),
			},
		)
		return
	}

	type normalizedStudent struct {
		Name       string
		Email      string
		Enrollment string
	}

	normalizedStudents := make(
		[]normalizedStudent,
		0,
		len(request.Students),
	)

	emails := make(
		[]string,
		0,
		len(request.Students),
	)

	enrollments := make(
		[]string,
		0,
		len(request.Students),
	)

	emailSet := make(map[string]struct{})
	enrollmentSet := make(map[string]struct{})

	for _, input := range request.Students {
		name := strings.TrimSpace(input.Name)
		email := strings.ToLower(
			strings.TrimSpace(input.Email),
		)
		enrollment := strings.ToUpper(
			strings.TrimSpace(input.Enrollment),
		)

		if name == "" || email == "" || enrollment == "" {
			c.JSON(
				http.StatusBadRequest,
				gin.H{
					"status":  "error",
					"message": "Todos los alumnos deben tener nombre, correo y matrícula",
				},
			)
			return
		}

		if _, exists := emailSet[email]; exists {
			c.JSON(
				http.StatusConflict,
				gin.H{
					"status":  "error",
					"message": "El correo está repetido dentro de la solicitud: " + email,
				},
			)
			return
		}

		if _, exists := enrollmentSet[enrollment]; exists {
			c.JSON(
				http.StatusConflict,
				gin.H{
					"status":  "error",
					"message": "La matrícula está repetida dentro de la solicitud: " + enrollment,
				},
			)
			return
		}

		emailSet[email] = struct{}{}
		enrollmentSet[enrollment] = struct{}{}

		emails = append(
			emails,
			email,
		)

		enrollments = append(
			enrollments,
			enrollment,
		)

		normalizedStudents = append(
			normalizedStudents,
			normalizedStudent{
				Name:       name,
				Email:      email,
				Enrollment: enrollment,
			},
		)
	}

	existingUsers, err := handler.users.CountDocuments(
		ctx,
		bson.M{
			"$or": bson.A{
				bson.M{
					"email": bson.M{
						"$in": emails,
					},
				},
				bson.M{
					"enrollment": bson.M{
						"$in": enrollments,
					},
				},
			},
		},
	)
	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "No fue posible validar los alumnos existentes",
			},
		)
		return
	}

	if existingUsers > 0 {
		c.JSON(
			http.StatusConflict,
			gin.H{
				"status":  "error",
				"message": "Uno o más correos o matrículas ya están registrados. Utiliza posteriormente el módulo histórico para reactivar alumnos existentes",
			},
		)
		return
	}

	credentials := make(
		[]dto.ActivationCredential,
		0,
		len(normalizedStudents),
	)

	createdUserIDs := make(
		[]bson.ObjectID,
		0,
		len(normalizedStudents),
	)

	createdMembershipIDs := make(
		[]bson.ObjectID,
		0,
		len(normalizedStudents),
	)

	for _, input := range normalizedStudents {
		activationCode, err :=
			security.GenerateActivationCode()

		if err != nil {
			handler.rollbackStudentBatch(
				createdUserIDs,
				createdMembershipIDs,
			)

			c.JSON(
				http.StatusInternalServerError,
				gin.H{
					"status":  "error",
					"message": "No fue posible generar los códigos de activación",
				},
			)
			return
		}

		activationCodeHash, err :=
			security.HashActivationCode(
				activationCode,
			)

		if err != nil {
			handler.rollbackStudentBatch(
				createdUserIDs,
				createdMembershipIDs,
			)

			c.JSON(
				http.StatusInternalServerError,
				gin.H{
					"status":  "error",
					"message": "No fue posible proteger el código de activación",
				},
			)
			return
		}

		expiresAt :=
			security.CalculateActivationCodeExpiration(
				now,
				crew.EndAt,
			)

		studentID := bson.NewObjectID()
		membershipID := bson.NewObjectID()

		student := models.User{
			ID:           studentID,
			Name:         input.Name,
			Email:        input.Email,
			Enrollment:   input.Enrollment,
			PasswordHash: "",
			Role:         models.RoleStudent,
			Status:       models.StatusInactive,
			CreatedAt:    now,
			UpdatedAt:    now,
		}

		membership := models.CrewMembership{
			ID:                      membershipID,
			CrewID:                  crew.ID,
			StudentID:               studentID,
			ValidFrom:               crew.StartAt.UTC(),
			ValidUntil:              crew.EndAt.UTC(),
			Status:                  models.MembershipStatusPending,
			ActivationCodeHash:      activationCodeHash,
			ActivationCodeExpiresAt: &expiresAt,
			CreatedBy:               currentUser.ID,
			CreatedAt:               now,
			UpdatedAt:               now,
		}

		_, err = handler.users.InsertOne(
			ctx,
			student,
		)
		if err != nil {
			handler.rollbackStudentBatch(
				createdUserIDs,
				createdMembershipIDs,
			)

			statusCode := http.StatusInternalServerError
			message := "No fue posible registrar al alumno"

			if mongo.IsDuplicateKeyError(err) {
				statusCode = http.StatusConflict
				message = "El correo o la matrícula ya están registrados"
			}

			c.JSON(
				statusCode,
				gin.H{
					"status":  "error",
					"message": message,
				},
			)
			return
		}

		createdUserIDs = append(
			createdUserIDs,
			studentID,
		)

		_, err = handler.memberships.InsertOne(
			ctx,
			membership,
		)
		if err != nil {
			handler.rollbackStudentBatch(
				createdUserIDs,
				createdMembershipIDs,
			)

			c.JSON(
				http.StatusInternalServerError,
				gin.H{
					"status":  "error",
					"message": "No fue posible crear la membresía del alumno",
				},
			)
			return
		}

		createdMembershipIDs = append(
			createdMembershipIDs,
			membershipID,
		)

		credentials = append(
			credentials,
			dto.ActivationCredential{
				StudentID:      studentID.Hex(),
				Name:           student.Name,
				Email:          student.Email,
				Enrollment:     student.Enrollment,
				ActivationCode: activationCode,
				ExpiresAt:      expiresAt,
			},
		)

		if handler.emailService != nil {
			if err := handler.emailService.SendActivationCode(
				c.Request.Context(),
				student.Name,
				student.Email,
				activationCode,
				expiresAt,
			); err != nil {
				c.Error(err)
			}
		}
	}

	c.JSON(
		http.StatusCreated,
		dto.BatchCreateStudentsResponse{
			Message:     "Alumnos registrados correctamente",
			CrewID:      crew.ID.Hex(),
			Registered:  len(credentials),
			Credentials: credentials,
		},
	)
}

// ListByCrew procesa:
//
// GET /api/v1/crews/:id/students
func (handler *StudentHandler) ListByCrew(
	c *gin.Context,
) {
	crew, ok := middleware.GetCurrentCrew(c)
	if !ok {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "No se pudo recuperar la cuadrilla",
			},
		)
		return
	}

	page, valid := parseStudentPositiveInteger(
		c.Query("page"),
		1,
	)
	if !valid {
		c.JSON(
			http.StatusBadRequest,
			gin.H{
				"status":  "error",
				"message": "page debe ser un entero mayor que cero",
			},
		)
		return
	}

	limit, valid := parseStudentPositiveInteger(
		c.Query("limit"),
		10,
	)
	if !valid || limit > 100 {
		c.JSON(
			http.StatusBadRequest,
			gin.H{
				"status":  "error",
				"message": "limit debe estar entre 1 y 100",
			},
		)
		return
	}

	statusText := strings.ToUpper(
		strings.TrimSpace(
			c.Query("status"),
		),
	)

	var statusFilter models.MembershipStatus

	if statusText != "" {
		statusFilter = models.MembershipStatus(
			statusText,
		)

		if !isValidMembershipStatus(statusFilter) {
			c.JSON(
				http.StatusBadRequest,
				gin.H{
					"status":  "error",
					"message": "El estado debe ser PENDING, ACTIVE, INACTIVE, EXPIRED o REVOKED",
				},
			)
			return
		}
	}

	search := strings.ToLower(
		strings.TrimSpace(
			c.Query("search"),
		),
	)

	ctx, cancel := context.WithTimeout(
		c.Request.Context(),
		10*time.Second,
	)
	defer cancel()

	filter := bson.M{
		"crewId": crew.ID,
	}

	if statusText != "" {
		filter["status"] = statusFilter
	}

	cursor, err := handler.memberships.Find(
		ctx,
		filter,
		options.Find().SetSort(
			bson.D{
				{
					Key:   "createdAt",
					Value: -1,
				},
			},
		),
	)
	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "No fue posible consultar las membresías",
			},
		)
		return
	}
	defer cursor.Close(ctx)

	var memberships []models.CrewMembership

	if err := cursor.All(
		ctx,
		&memberships,
	); err != nil {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "No fue posible procesar las membresías",
			},
		)
		return
	}

	studentsMap, err := handler.loadStudents(
		ctx,
		memberships,
	)
	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "No fue posible recuperar los alumnos",
			},
		)
		return
	}

	type studentWithMembership struct {
		Student    models.User
		Membership models.CrewMembership
	}

	filtered := make(
		[]studentWithMembership,
		0,
		len(memberships),
	)

	for _, membership := range memberships {
		student, exists := studentsMap[membership.StudentID]
		if !exists {
			continue
		}

		if search != "" {
			searchable := strings.ToLower(
				student.Name + " " +
					student.Email + " " +
					student.Enrollment,
			)

			if !strings.Contains(
				searchable,
				search,
			) {
				continue
			}
		}

		filtered = append(
			filtered,
			studentWithMembership{
				Student:    student,
				Membership: membership,
			},
		)
	}

	total := len(filtered)
	start := (page - 1) * limit

	if start > total {
		start = total
	}

	end := start + limit

	if end > total {
		end = total
	}

	responses := make(
		[]dto.StudentResponse,
		0,
		end-start,
	)

	for _, item := range filtered[start:end] {
		membership := item.Membership

		responses = append(
			responses,
			dto.NewStudentResponse(
				item.Student,
				&membership,
				crew,
			),
		)
	}

	totalPages := int64(0)

	if total > 0 {
		totalPages = int64(
			(total + limit - 1) / limit,
		)
	}

	c.JSON(
		http.StatusOK,
		dto.StudentListResponse{
			Students: responses,
			Pagination: dto.PaginationResponse{
				Page:       page,
				Limit:      limit,
				Total:      int64(total),
				TotalPages: totalPages,
			},
		},
	)
}

// GetByID procesa:
//
// GET /api/v1/students/:id
func (handler *StudentHandler) GetByID(
	c *gin.Context,
) {
	studentID, ok := studentIDFromParam(c)
	if !ok {
		return
	}

	ctx, cancel := context.WithTimeout(
		c.Request.Context(),
		8*time.Second,
	)
	defer cancel()

	var student models.User

	err := handler.users.FindOne(
		ctx,
		bson.M{
			"_id":  studentID,
			"role": models.RoleStudent,
		},
	).Decode(&student)

	if errors.Is(err, mongo.ErrNoDocuments) {
		c.JSON(
			http.StatusNotFound,
			gin.H{
				"status":  "error",
				"message": "El alumno no existe",
			},
		)
		return
	}

	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "No fue posible consultar al alumno",
			},
		)
		return
	}

	membership, membershipErr :=
		handler.findLatestMembership(
			ctx,
			studentID,
		)

	var crew *models.Crew

	if membershipErr == nil {
		crew, err = handler.findCrew(
			ctx,
			membership.CrewID,
		)
		if err != nil &&
			!errors.Is(err, mongo.ErrNoDocuments) {
			c.JSON(
				http.StatusInternalServerError,
				gin.H{
					"status":  "error",
					"message": "No fue posible consultar la cuadrilla del alumno",
				},
			)
			return
		}
	}

	currentUser, authenticated :=
		middleware.GetCurrentUser(c)

	if !authenticated {
		c.JSON(
			http.StatusUnauthorized,
			gin.H{
				"status":  "error",
				"message": "No se encontró una sesión válida",
			},
		)
		return
	}

	if currentUser.Role == models.RoleManager {
		if crew == nil ||
			crew.ManagerID != currentUser.ID {
			c.JSON(
				http.StatusForbidden,
				gin.H{
					"status":  "error",
					"message": "No tienes acceso a este alumno",
				},
			)
			return
		}
	}

	var membershipPointer *models.CrewMembership

	if membershipErr == nil {
		membershipPointer = membership
	}

	c.JSON(
		http.StatusOK,
		gin.H{
			"student": dto.NewStudentResponse(
				student,
				membershipPointer,
				crew,
			),
		},
	)
}

// Update procesa:
//
// PUT /api/v1/students/:id
func (handler *StudentHandler) Update(
	c *gin.Context,
) {
	studentID, ok := studentIDFromParam(c)
	if !ok {
		return
	}

	var request dto.UpdateStudentRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(
			http.StatusBadRequest,
			gin.H{
				"status":  "error",
				"message": "Los datos del alumno no son válidos",
				"details": err.Error(),
			},
		)
		return
	}

	name := strings.TrimSpace(request.Name)
	email := strings.ToLower(
		strings.TrimSpace(request.Email),
	)
	enrollment := strings.ToUpper(
		strings.TrimSpace(request.Enrollment),
	)

	ctx, cancel := context.WithTimeout(
		c.Request.Context(),
		10*time.Second,
	)
	defer cancel()

	result, err := handler.users.UpdateOne(
		ctx,
		bson.M{
			"_id":  studentID,
			"role": models.RoleStudent,
		},
		bson.M{
			"$set": bson.M{
				"name":       name,
				"email":      email,
				"enrollment": enrollment,
				"updatedAt":  time.Now().UTC(),
			},
		},
	)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			c.JSON(
				http.StatusConflict,
				gin.H{
					"status":  "error",
					"message": "El correo o la matrícula ya están registrados",
				},
			)
			return
		}

		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "No fue posible actualizar al alumno",
			},
		)
		return
	}

	if result.MatchedCount == 0 {
		c.JSON(
			http.StatusNotFound,
			gin.H{
				"status":  "error",
				"message": "El alumno no existe",
			},
		)
		return
	}

	student, membership, crew, err :=
		handler.loadCompleteStudent(
			ctx,
			studentID,
		)
	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "El alumno se actualizó, pero no pudo recuperarse",
			},
		)
		return
	}

	c.JSON(
		http.StatusOK,
		gin.H{
			"message": "Alumno actualizado correctamente",
			"student": dto.NewStudentResponse(
				*student,
				membership,
				crew,
			),
		},
	)
}

// UpdateStatus procesa:
//
// PATCH /api/v1/students/:id/status
func (handler *StudentHandler) UpdateStatus(
	c *gin.Context,
) {
	studentID, ok := studentIDFromParam(c)
	if !ok {
		return
	}

	var request dto.UpdateStudentStatusRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(
			http.StatusBadRequest,
			gin.H{
				"status":  "error",
				"message": "Debes enviar el nuevo estado",
			},
		)
		return
	}

	status := models.UserStatus(
		strings.ToUpper(
			strings.TrimSpace(
				string(request.Status),
			),
		),
	)

	if !isValidStudentUserStatus(status) {
		c.JSON(
			http.StatusBadRequest,
			gin.H{
				"status":  "error",
				"message": "El estado debe ser ACTIVE, INACTIVE o BLOCKED",
			},
		)
		return
	}

	ctx, cancel := context.WithTimeout(
		c.Request.Context(),
		10*time.Second,
	)
	defer cancel()

	if status == models.StatusActive {
		membership, err :=
			handler.findLatestMembership(
				ctx,
				studentID,
			)

		if err != nil ||
			membership.Status != models.MembershipStatusActive ||
			!membership.ValidUntil.After(time.Now().UTC()) {
			c.JSON(
				http.StatusConflict,
				gin.H{
					"status":  "error",
					"message": "El alumno debe activar primero una membresía vigente",
				},
			)
			return
		}
	}

	result, err := handler.users.UpdateOne(
		ctx,
		bson.M{
			"_id":  studentID,
			"role": models.RoleStudent,
		},
		bson.M{
			"$set": bson.M{
				"status":    status,
				"updatedAt": time.Now().UTC(),
			},
		},
	)
	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "No fue posible actualizar el estado",
			},
		)
		return
	}

	if result.MatchedCount == 0 {
		c.JSON(
			http.StatusNotFound,
			gin.H{
				"status":  "error",
				"message": "El alumno no existe",
			},
		)
		return
	}

	student, membership, crew, err :=
		handler.loadCompleteStudent(
			ctx,
			studentID,
		)
	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "El estado cambió, pero el alumno no pudo recuperarse",
			},
		)
		return
	}

	c.JSON(
		http.StatusOK,
		gin.H{
			"message": "Estado del alumno actualizado correctamente",
			"student": dto.NewStudentResponse(
				*student,
				membership,
				crew,
			),
		},
	)
}

// NewActivationCode procesa:
//
// POST /api/v1/students/:id/new-activation-code
func (handler *StudentHandler) NewActivationCode(
	c *gin.Context,
) {
	studentID, ok := studentIDFromParam(c)
	if !ok {
		return
	}

	ctx, cancel := context.WithTimeout(
		c.Request.Context(),
		10*time.Second,
	)
	defer cancel()

	var student models.User

	err := handler.users.FindOne(
		ctx,
		bson.M{
			"_id":  studentID,
			"role": models.RoleStudent,
		},
	).Decode(&student)

	if errors.Is(err, mongo.ErrNoDocuments) {
		c.JSON(
			http.StatusNotFound,
			gin.H{
				"status":  "error",
				"message": "El alumno no existe",
			},
		)
		return
	}

	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "No fue posible consultar al alumno",
			},
		)
		return
	}

	now := time.Now().UTC()

	var membership models.CrewMembership

	err = handler.memberships.FindOne(
		ctx,
		bson.M{
			"studentId": studentID,
			"status": bson.M{
				"$in": bson.A{
					models.MembershipStatusPending,
					models.MembershipStatusActive,
					models.MembershipStatusInactive,
				},
			},
			"validUntil": bson.M{
				"$gt": now,
			},
		},
		options.FindOne().SetSort(
			bson.D{
				{
					Key:   "createdAt",
					Value: -1,
				},
			},
		),
	).Decode(&membership)

	if errors.Is(err, mongo.ErrNoDocuments) {
		c.JSON(
			http.StatusConflict,
			gin.H{
				"status":  "error",
				"message": "El alumno no tiene una membresía vigente",
			},
		)
		return
	}

	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "No fue posible consultar la membresía",
			},
		)
		return
	}

	crew, err := handler.findCrew(
		ctx,
		membership.CrewID,
	)
	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "No fue posible consultar la cuadrilla",
			},
		)
		return
	}

	if crew.Status == models.CrewStatusFinished ||
		crew.Status == models.CrewStatusCancelled ||
		!crew.EndAt.After(now) {
		c.JSON(
			http.StatusConflict,
			gin.H{
				"status":  "error",
				"message": "La cuadrilla ya no permite generar accesos",
			},
		)
		return
	}

	activationCode, err :=
		security.GenerateActivationCode()
	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "No fue posible generar el código",
			},
		)
		return
	}

	codeHash, err := security.HashActivationCode(
		activationCode,
	)
	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "No fue posible proteger el código",
			},
		)
		return
	}

	expiresAt :=
		security.CalculateActivationCodeExpiration(
			now,
			crew.EndAt,
		)

	_, err = handler.memberships.UpdateOne(
		ctx,
		bson.M{
			"_id": membership.ID,
		},
		bson.M{
			"$set": bson.M{
				"status":                  models.MembershipStatusPending,
				"activationCodeHash":      codeHash,
				"activationCodeExpiresAt": expiresAt,
				"updatedAt":               now,
			},
			"$unset": bson.M{
				"activatedAt": "",
				"archivedAt":  "",
			},
		},
	)
	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "No fue posible guardar el nuevo código",
			},
		)
		return
	}

	_, err = handler.users.UpdateOne(
		ctx,
		bson.M{
			"_id": studentID,
		},
		bson.M{
			"$set": bson.M{
				"status":    models.StatusInactive,
				"updatedAt": now,
			},
		},
	)
	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "El código fue generado, pero no pudo desactivarse temporalmente la cuenta",
			},
		)
		return
	}

	c.JSON(
		http.StatusOK,
		gin.H{
			"message": "Nuevo código generado correctamente",
			"credential": dto.ActivationCredential{
				StudentID:      student.ID.Hex(),
				Name:           student.Name,
				Email:          student.Email,
				Enrollment:     student.Enrollment,
				ActivationCode: activationCode,
				ExpiresAt:      expiresAt,
			},
		},
	)

	if handler.emailService != nil {
		if err := handler.emailService.SendActivationCode(
			c.Request.Context(),
			student.Name,
			student.Email,
			activationCode,
			expiresAt,
		); err != nil {
			c.Error(err)
		}
	}
}

// Activate procesa:
//
// POST /api/v1/auth/activate
func (handler *StudentHandler) Activate(
	c *gin.Context,
) {
	var request dto.ActivateStudentRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(
			http.StatusBadRequest,
			gin.H{
				"status":  "error",
				"message": "Los datos de activación no son válidos",
				"details": err.Error(),
			},
		)
		return
	}

	email := strings.ToLower(
		strings.TrimSpace(request.Email),
	)

	ctx, cancel := context.WithTimeout(
		c.Request.Context(),
		15*time.Second,
	)
	defer cancel()

	var student models.User

	err := handler.users.FindOne(
		ctx,
		bson.M{
			"email": email,
			"role":  models.RoleStudent,
		},
	).Decode(&student)

	if errors.Is(err, mongo.ErrNoDocuments) {
		invalidActivationResponse(c)
		return
	}

	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "No fue posible validar la activación",
			},
		)
		return
	}

	now := time.Now().UTC()

	cursor, err := handler.memberships.Find(
		ctx,
		bson.M{
			"studentId": student.ID,
			"status":    models.MembershipStatusPending,
			"validUntil": bson.M{
				"$gt": now,
			},
			"activationCodeExpiresAt": bson.M{
				"$gt": now,
			},
		},
		options.Find().SetSort(
			bson.D{
				{
					Key:   "createdAt",
					Value: -1,
				},
			},
		),
	)
	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "No fue posible consultar las activaciones",
			},
		)
		return
	}
	defer cursor.Close(ctx)

	var memberships []models.CrewMembership

	if err := cursor.All(
		ctx,
		&memberships,
	); err != nil {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "No fue posible procesar la activación",
			},
		)
		return
	}

	var selectedMembership *models.CrewMembership

	for index := range memberships {
		if security.CheckActivationCode(
			memberships[index].ActivationCodeHash,
			request.ActivationCode,
		) {
			selectedMembership = &memberships[index]
			break
		}
	}

	if selectedMembership == nil {
		invalidActivationResponse(c)
		return
	}

	crew, err := handler.findCrew(
		ctx,
		selectedMembership.CrewID,
	)
	if err != nil ||
		crew.Status == models.CrewStatusFinished ||
		crew.Status == models.CrewStatusCancelled ||
		!crew.EndAt.After(now) {
		c.JSON(
			http.StatusConflict,
			gin.H{
				"status":  "error",
				"message": "La cuadrilla asociada ya no está disponible",
			},
		)
		return
	}

	passwordHash, err := security.HashPassword(
		request.Password,
	)
	if err != nil {
		c.JSON(
			http.StatusBadRequest,
			gin.H{
				"status":  "error",
				"message": err.Error(),
			},
		)
		return
	}

	oldStatus := student.Status
	oldPasswordHash := student.PasswordHash

	_, err = handler.users.UpdateOne(
		ctx,
		bson.M{
			"_id": student.ID,
		},
		bson.M{
			"$set": bson.M{
				"passwordHash": passwordHash,
				"status":       models.StatusActive,
				"updatedAt":    now,
			},
		},
	)
	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "No fue posible activar la cuenta",
			},
		)
		return
	}

	_, err = handler.memberships.UpdateOne(
		ctx,
		bson.M{
			"_id": selectedMembership.ID,
		},
		bson.M{
			"$set": bson.M{
				"status":      models.MembershipStatusActive,
				"activatedAt": now,
				"updatedAt":   now,
			},
			"$unset": bson.M{
				"activationCodeHash":      "",
				"activationCodeExpiresAt": "",
			},
		},
	)
	if err != nil {
		// Restauración de mejor esfuerzo.
		_, _ = handler.users.UpdateOne(
			context.Background(),
			bson.M{
				"_id": student.ID,
			},
			bson.M{
				"$set": bson.M{
					"passwordHash": oldPasswordHash,
					"status":       oldStatus,
				},
			},
		)

		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "La cuenta no pudo completar su activación",
			},
		)
		return
	}

	student.PasswordHash = passwordHash
	student.Status = models.StatusActive
	student.UpdatedAt = now

	selectedMembership.Status =
		models.MembershipStatusActive
	selectedMembership.ActivatedAt = &now
	selectedMembership.UpdatedAt = now
	selectedMembership.ActivationCodeHash = ""
	selectedMembership.ActivationCodeExpiresAt = nil

	c.JSON(
		http.StatusOK,
		gin.H{
			"message": "Cuenta activada correctamente. Ya puedes iniciar sesión",
			"student": dto.NewStudentResponse(
				student,
				selectedMembership,
				crew,
			),
		},
	)
}

func (handler *StudentHandler) findLatestMembership(
	ctx context.Context,
	studentID bson.ObjectID,
) (*models.CrewMembership, error) {
	var membership models.CrewMembership

	err := handler.memberships.FindOne(
		ctx,
		bson.M{
			"studentId": studentID,
		},
		options.FindOne().SetSort(
			bson.D{
				{
					Key:   "createdAt",
					Value: -1,
				},
			},
		),
	).Decode(&membership)

	if err != nil {
		return nil, err
	}

	return &membership, nil
}

func (handler *StudentHandler) findCrew(
	ctx context.Context,
	crewID bson.ObjectID,
) (*models.Crew, error) {
	var crew models.Crew

	err := handler.crews.FindOne(
		ctx,
		bson.M{
			"_id": crewID,
		},
	).Decode(&crew)

	if err != nil {
		return nil, err
	}

	return &crew, nil
}

func (handler *StudentHandler) loadCompleteStudent(
	ctx context.Context,
	studentID bson.ObjectID,
) (
	*models.User,
	*models.CrewMembership,
	*models.Crew,
	error,
) {
	var student models.User

	err := handler.users.FindOne(
		ctx,
		bson.M{
			"_id":  studentID,
			"role": models.RoleStudent,
		},
	).Decode(&student)
	if err != nil {
		return nil, nil, nil, err
	}

	membership, err :=
		handler.findLatestMembership(
			ctx,
			studentID,
		)

	if errors.Is(err, mongo.ErrNoDocuments) {
		return &student, nil, nil, nil
	}

	if err != nil {
		return nil, nil, nil, err
	}

	crew, err := handler.findCrew(
		ctx,
		membership.CrewID,
	)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return &student, membership, nil, nil
	}

	if err != nil {
		return nil, nil, nil, err
	}

	return &student, membership, crew, nil
}

func (handler *StudentHandler) loadStudents(
	ctx context.Context,
	memberships []models.CrewMembership,
) (map[bson.ObjectID]models.User, error) {
	result := make(
		map[bson.ObjectID]models.User,
	)

	if len(memberships) == 0 {
		return result, nil
	}

	studentIDs := make(
		[]bson.ObjectID,
		0,
		len(memberships),
	)

	for _, membership := range memberships {
		studentIDs = append(
			studentIDs,
			membership.StudentID,
		)
	}

	cursor, err := handler.users.Find(
		ctx,
		bson.M{
			"_id": bson.M{
				"$in": studentIDs,
			},
			"role": models.RoleStudent,
		},
	)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var students []models.User

	if err := cursor.All(
		ctx,
		&students,
	); err != nil {
		return nil, err
	}

	for _, student := range students {
		result[student.ID] = student
	}

	return result, nil
}

func (handler *StudentHandler) rollbackStudentBatch(
	userIDs []bson.ObjectID,
	membershipIDs []bson.ObjectID,
) {
	ctx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	if len(membershipIDs) > 0 {
		_, _ = handler.memberships.DeleteMany(
			ctx,
			bson.M{
				"_id": bson.M{
					"$in": membershipIDs,
				},
			},
		)
	}

	if len(userIDs) > 0 {
		_, _ = handler.users.DeleteMany(
			ctx,
			bson.M{
				"_id": bson.M{
					"$in": userIDs,
				},
			},
		)
	}
}

func studentIDFromParam(
	c *gin.Context,
) (bson.ObjectID, bool) {
	idText := strings.TrimSpace(
		c.Param("id"),
	)

	studentID, err := bson.ObjectIDFromHex(
		idText,
	)
	if err != nil {
		c.JSON(
			http.StatusBadRequest,
			gin.H{
				"status":  "error",
				"message": "El identificador del alumno no es válido",
			},
		)

		return bson.ObjectID{}, false
	}

	return studentID, true
}

func isValidMembershipStatus(
	status models.MembershipStatus,
) bool {
	switch status {
	case models.MembershipStatusPending,
		models.MembershipStatusActive,
		models.MembershipStatusInactive,
		models.MembershipStatusExpired,
		models.MembershipStatusRevoked:
		return true

	default:
		return false
	}
}

func isValidStudentUserStatus(
	status models.UserStatus,
) bool {
	switch status {
	case models.StatusActive,
		models.StatusInactive,
		models.StatusBlocked:
		return true

	default:
		return false
	}
}

func parseStudentPositiveInteger(
	value string,
	defaultValue int,
) (int, bool) {
	value = strings.TrimSpace(value)

	if value == "" {
		return defaultValue, true
	}

	number, err := strconv.Atoi(value)

	if err != nil || number < 1 {
		return 0, false
	}

	return number, true
}

func invalidActivationResponse(
	c *gin.Context,
) {
	c.JSON(
		http.StatusUnauthorized,
		gin.H{
			"status":  "error",
			"message": "El correo o el código de activación son incorrectos o han expirado",
		},
	)
}
