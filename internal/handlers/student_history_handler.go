package handlers

import (
	"context"
	"errors"
	"net/http"
	"regexp"
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

// StudentHistoryHandler contiene las colecciones utilizadas
// por el módulo de historial y reactivación.
type StudentHistoryHandler struct {
	users        *mongo.Collection
	crews        *mongo.Collection
	memberships  *mongo.Collection
	emailService *services.EmailService
}

// NewStudentHistoryHandler crea el handler del módulo.
func NewStudentHistoryHandler(
	mongodb *database.MongoDB,
	emailService *services.EmailService,
) *StudentHistoryHandler {
	return &StudentHistoryHandler{
		users:        mongodb.Collection("users"),
		crews:        mongodb.Collection("crews"),
		memberships:  mongodb.Collection("crew_memberships"),
		emailService: emailService,
	}
}

// List procesa:
//
// GET /api/v1/students/history
//
// Parámetros:
//
// ?page=1
// ?limit=10
// ?search=correo-matricula-nombre
// ?status=ACTIVE
func (handler *StudentHistoryHandler) List(
	c *gin.Context,
) {
	page, valid := parseStudentHistoryPositiveInteger(
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

	limit, valid := parseStudentHistoryPositiveInteger(
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

	filter := bson.M{
		"role": models.RoleStudent,
	}

	search := strings.TrimSpace(
		c.Query("search"),
	)

	if search != "" {
		safeSearch := regexp.QuoteMeta(search)

		filter["$or"] = bson.A{
			bson.M{
				"name": bson.M{
					"$regex":   safeSearch,
					"$options": "i",
				},
			},
			bson.M{
				"email": bson.M{
					"$regex":   safeSearch,
					"$options": "i",
				},
			},
			bson.M{
				"enrollment": bson.M{
					"$regex":   safeSearch,
					"$options": "i",
				},
			},
		}
	}

	statusText := strings.ToUpper(
		strings.TrimSpace(
			c.Query("status"),
		),
	)

	if statusText != "" {
		switch statusText {
		case string(models.StatusActive),
			string(models.StatusInactive),
			string(models.StatusBlocked):

			filter["status"] = statusText

		default:
			c.JSON(
				http.StatusBadRequest,
				gin.H{
					"status":  "error",
					"message": "El estado debe ser ACTIVE, INACTIVE o BLOCKED",
				},
			)
			return
		}
	}

	ctx, cancel := context.WithTimeout(
		c.Request.Context(),
		15*time.Second,
	)
	defer cancel()

	total, err := handler.users.CountDocuments(
		ctx,
		filter,
	)
	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "No fue posible contar los alumnos",
			},
		)
		return
	}

	skip := int64(
		(page - 1) * limit,
	)

	cursor, err := handler.users.Find(
		ctx,
		filter,
		options.Find().
			SetSort(
				bson.D{
					{
						Key:   "createdAt",
						Value: -1,
					},
				},
			).
			SetSkip(skip).
			SetLimit(int64(limit)),
	)
	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "No fue posible consultar los alumnos",
			},
		)
		return
	}
	defer cursor.Close(ctx)

	students := make(
		[]models.User,
		0,
	)

	if err := cursor.All(
		ctx,
		&students,
	); err != nil {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "No fue posible procesar los alumnos",
			},
		)
		return
	}

	membershipsByStudent, allMemberships, err :=
		handler.loadMembershipsByStudents(
			ctx,
			students,
		)

	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "No fue posible recuperar el historial de membresías",
			},
		)
		return
	}

	crewsMap, err := handler.loadMembershipCrews(
		ctx,
		allMemberships,
	)
	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "No fue posible recuperar las cuadrillas",
			},
		)
		return
	}

	responses := make(
		[]dto.StudentHistoryListItemResponse,
		0,
		len(students),
	)

	for _, student := range students {
		studentMemberships :=
			membershipsByStudent[student.ID]

		item := dto.StudentHistoryListItemResponse{
			Student: dto.NewStudentHistoryStudentResponse(
				student,
			),
			TotalMemberships: len(studentMemberships),
		}

		if len(studentMemberships) > 0 {
			latest := studentMemberships[0]

			var crewPointer *models.Crew

			crew, exists := crewsMap[latest.CrewID]
			if exists {
				crewCopy := crew
				crewPointer = &crewCopy
			}

			latestResponse :=
				dto.NewMembershipHistoryResponse(
					latest,
					crewPointer,
				)

			item.LatestMembership = &latestResponse
		}

		responses = append(
			responses,
			item,
		)
	}

	totalPages := int64(0)

	if total > 0 {
		totalPages = (total + int64(limit) - 1) / int64(limit)
	}

	c.JSON(
		http.StatusOK,
		dto.StudentHistoryListResponse{
			Students: responses,
			Pagination: dto.PaginationResponse{
				Page:       page,
				Limit:      limit,
				Total:      total,
				TotalPages: totalPages,
			},
		},
	)
}

// GetMemberships procesa:
//
// GET /api/v1/students/:id/memberships
func (handler *StudentHistoryHandler) GetMemberships(
	c *gin.Context,
) {
	studentID, ok := studentHistoryObjectIDFromParam(
		c,
		"id",
		"El identificador del alumno no es válido",
	)

	if !ok {
		return
	}

	ctx, cancel := context.WithTimeout(
		c.Request.Context(),
		12*time.Second,
	)
	defer cancel()

	student, err := handler.findHistoryStudent(
		ctx,
		studentID,
	)

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

	memberships, err := handler.findStudentMemberships(
		ctx,
		student.ID,
	)

	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "No fue posible consultar el historial del alumno",
			},
		)
		return
	}

	crewsMap, err := handler.loadMembershipCrews(
		ctx,
		memberships,
	)

	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "No fue posible recuperar las cuadrillas",
			},
		)
		return
	}

	responses := make(
		[]dto.MembershipHistoryResponse,
		0,
		len(memberships),
	)

	for _, membership := range memberships {
		var crewPointer *models.Crew

		crew, exists := crewsMap[membership.CrewID]
		if exists {
			crewCopy := crew
			crewPointer = &crewCopy
		}

		responses = append(
			responses,
			dto.NewMembershipHistoryResponse(
				membership,
				crewPointer,
			),
		)
	}

	c.JSON(
		http.StatusOK,
		dto.StudentMembershipHistoryResponse{
			Student: dto.NewStudentHistoryStudentResponse(
				*student,
			),
			Memberships: responses,
			Total:       len(responses),
		},
	)
}

// Reactivate procesa:
//
// POST /api/v1/crews/:id/students/:studentId/reactivate
//
// Incorpora un alumno existente a una cuadrilla nueva.
func (handler *StudentHistoryHandler) Reactivate(
	c *gin.Context,
) {
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

	crewID, ok := studentHistoryObjectIDFromParam(
		c,
		"id",
		"El identificador de la cuadrilla no es válido",
	)

	if !ok {
		return
	}

	studentID, ok := studentHistoryObjectIDFromParam(
		c,
		"studentId",
		"El identificador del alumno no es válido",
	)

	if !ok {
		return
	}

	ctx, cancel := context.WithTimeout(
		c.Request.Context(),
		15*time.Second,
	)
	defer cancel()

	crew, err := handler.findHistoryCrew(
		ctx,
		crewID,
	)

	if errors.Is(err, mongo.ErrNoDocuments) {
		c.JSON(
			http.StatusNotFound,
			gin.H{
				"status":  "error",
				"message": "La cuadrilla no existe",
			},
		)
		return
	}

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

	student, err := handler.findHistoryStudent(
		ctx,
		studentID,
	)

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

	if err := validateStudentReactivation(
		student,
		crew,
		now,
	); err != nil {
		c.JSON(
			http.StatusConflict,
			gin.H{
				"status":  "error",
				"message": err.Error(),
			},
		)
		return
	}

	// El índice actual crewId + studentId es único.
	// Por ello no se puede crear una segunda membresía
	// del mismo alumno dentro de la misma cuadrilla.
	existingTargetMembership, err :=
		handler.findMembershipByCrewAndStudent(
			ctx,
			crew.ID,
			student.ID,
		)

	if err == nil && existingTargetMembership != nil {
		c.JSON(
			http.StatusConflict,
			gin.H{
				"status":  "error",
				"message": "El alumno ya tiene un registro histórico en esta cuadrilla",
			},
		)
		return
	}

	if err != nil &&
		!errors.Is(err, mongo.ErrNoDocuments) {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "No fue posible validar el historial del alumno",
			},
		)
		return
	}

	// No se permite asignar al alumno a otra cuadrilla
	// si ya tiene una membresía pendiente o activa vigente.
	activeMembership, err :=
		handler.findCurrentStudentMembership(
			ctx,
			student.ID,
			now,
		)

	if err == nil && activeMembership != nil {
		c.JSON(
			http.StatusConflict,
			gin.H{
				"status":       "error",
				"message":      "El alumno ya tiene una membresía pendiente o activa vigente",
				"membershipId": activeMembership.ID.Hex(),
				"crewId":       activeMembership.CrewID.Hex(),
			},
		)
		return
	}

	if err != nil &&
		!errors.Is(err, mongo.ErrNoDocuments) {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "No fue posible validar la membresía vigente",
			},
		)
		return
	}

	occupiedPlaces, err :=
		handler.countOccupiedCrewPlaces(
			ctx,
			crew.ID,
			now,
		)

	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "No fue posible consultar el cupo de la cuadrilla",
			},
		)
		return
	}

	if occupiedPlaces >= int64(crew.StudentLimit) {
		c.JSON(
			http.StatusConflict,
			gin.H{
				"status":         "error",
				"message":        "La cuadrilla ya alcanzó el límite de alumnos",
				"studentLimit":   crew.StudentLimit,
				"occupiedPlaces": occupiedPlaces,
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
				"message": "No fue posible generar el código de activación",
			},
		)
		return
	}

	activationCodeHash, err :=
		security.HashActivationCode(
			activationCode,
		)

	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "No fue posible proteger el código de activación",
			},
		)
		return
	}

	activationExpiresAt :=
		security.CalculateActivationCodeExpiration(
			now,
			crew.EndAt,
		)

	if !activationExpiresAt.After(now) {
		c.JSON(
			http.StatusConflict,
			gin.H{
				"status":  "error",
				"message": "La cuadrilla no tiene vigencia suficiente para generar una activación",
			},
		)
		return
	}

	validFrom := crew.StartAt

	if now.After(validFrom) {
		validFrom = now
	}

	membership := models.CrewMembership{
		ID:        bson.NewObjectID(),
		CrewID:    crew.ID,
		StudentID: student.ID,

		ValidFrom:  validFrom,
		ValidUntil: crew.EndAt,

		Status: models.MembershipStatusPending,

		ActivationCodeHash: activationCodeHash,

		ActivationCodeExpiresAt: &activationExpiresAt,

		CreatedBy: currentUser.ID,
		CreatedAt: now,
		UpdatedAt: now,
	}

	_, err = handler.memberships.InsertOne(
		ctx,
		membership,
	)

	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			c.JSON(
				http.StatusConflict,
				gin.H{
					"status":  "error",
					"message": "El alumno ya está relacionado con esta cuadrilla",
				},
			)
			return
		}

		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "No fue posible crear la nueva membresía",
			},
		)
		return
	}

	// El alumno permanece inactivo hasta utilizar
	// el nuevo código de activación.
	updateResult, err := handler.users.UpdateOne(
		ctx,
		bson.M{
			"_id":  student.ID,
			"role": models.RoleStudent,
			"status": bson.M{
				"$ne": models.StatusBlocked,
			},
		},
		bson.M{
			"$set": bson.M{
				"status":    models.StatusInactive,
				"updatedAt": now,
			},
		},
	)

	if err != nil || updateResult.MatchedCount == 0 {
		// Rollback para evitar una membresía sin actualizar
		// el estado de la cuenta.
		_, _ = handler.memberships.DeleteOne(
			context.Background(),
			bson.M{
				"_id": membership.ID,
			},
		)

		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "No fue posible actualizar el estado del alumno",
			},
		)
		return
	}

	student.Status = models.StatusInactive
	student.UpdatedAt = now

	membershipResponse :=
		dto.NewMembershipHistoryResponse(
			membership,
			crew,
		)

	c.JSON(
		http.StatusCreated,
		gin.H{
			"message": "Alumno incorporado a la nueva cuadrilla",
			"data": dto.ReactivateStudentResponse{
				Student: dto.NewStudentHistoryStudentResponse(
					*student,
				),
				Membership: membershipResponse,
				Credential: dto.ReactivationCredentialResponse{
					Email:          student.Email,
					ActivationCode: activationCode,
					ExpiresAt:      activationExpiresAt,
				},
			},
		},
	)

	if handler.emailService != nil {
		if err := handler.emailService.SendActivationCode(
			c.Request.Context(),
			student.Name,
			student.Email,
			activationCode,
			activationExpiresAt,
		); err != nil {
			c.Error(err)
		}
	}
}

func (handler *StudentHistoryHandler) findHistoryStudent(
	ctx context.Context,
	studentID bson.ObjectID,
) (*models.User, error) {
	var student models.User

	err := handler.users.FindOne(
		ctx,
		bson.M{
			"_id":  studentID,
			"role": models.RoleStudent,
		},
	).Decode(&student)

	if err != nil {
		return nil, err
	}

	return &student, nil
}

func (handler *StudentHistoryHandler) findHistoryCrew(
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

func (handler *StudentHistoryHandler) findStudentMemberships(
	ctx context.Context,
	studentID bson.ObjectID,
) ([]models.CrewMembership, error) {
	cursor, err := handler.memberships.Find(
		ctx,
		bson.M{
			"studentId": studentID,
		},
		options.Find().
			SetSort(
				bson.D{
					{
						Key:   "validFrom",
						Value: -1,
					},
					{
						Key:   "createdAt",
						Value: -1,
					},
				},
			),
	)

	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	memberships := make(
		[]models.CrewMembership,
		0,
	)

	if err := cursor.All(
		ctx,
		&memberships,
	); err != nil {
		return nil, err
	}

	return memberships, nil
}

func (
	handler *StudentHistoryHandler,
) findMembershipByCrewAndStudent(
	ctx context.Context,
	crewID bson.ObjectID,
	studentID bson.ObjectID,
) (*models.CrewMembership, error) {
	var membership models.CrewMembership

	err := handler.memberships.FindOne(
		ctx,
		bson.M{
			"crewId":    crewID,
			"studentId": studentID,
		},
	).Decode(&membership)

	if err != nil {
		return nil, err
	}

	return &membership, nil
}

func (
	handler *StudentHistoryHandler,
) findCurrentStudentMembership(
	ctx context.Context,
	studentID bson.ObjectID,
	now time.Time,
) (*models.CrewMembership, error) {
	var membership models.CrewMembership

	err := handler.memberships.FindOne(
		ctx,
		bson.M{
			"studentId": studentID,
			"status": bson.M{
				"$in": bson.A{
					models.MembershipStatusPending,
					models.MembershipStatusActive,
				},
			},
			"validUntil": bson.M{
				"$gt": now,
			},
		},
		options.FindOne().
			SetSort(
				bson.D{
					{
						Key:   "validUntil",
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

func (
	handler *StudentHistoryHandler,
) countOccupiedCrewPlaces(
	ctx context.Context,
	crewID bson.ObjectID,
	now time.Time,
) (int64, error) {
	return handler.memberships.CountDocuments(
		ctx,
		bson.M{
			"crewId": crewID,
			"status": bson.M{
				"$in": bson.A{
					models.MembershipStatusPending,
					models.MembershipStatusActive,
				},
			},
			"validUntil": bson.M{
				"$gt": now,
			},
		},
	)
}

func (
	handler *StudentHistoryHandler,
) loadMembershipsByStudents(
	ctx context.Context,
	students []models.User,
) (
	map[bson.ObjectID][]models.CrewMembership,
	[]models.CrewMembership,
	error,
) {
	result := make(
		map[bson.ObjectID][]models.CrewMembership,
	)

	if len(students) == 0 {
		return result, []models.CrewMembership{}, nil
	}

	studentIDs := make(
		[]bson.ObjectID,
		0,
		len(students),
	)

	for _, student := range students {
		studentIDs = append(
			studentIDs,
			student.ID,
		)
	}

	cursor, err := handler.memberships.Find(
		ctx,
		bson.M{
			"studentId": bson.M{
				"$in": studentIDs,
			},
		},
		options.Find().
			SetSort(
				bson.D{
					{
						Key:   "studentId",
						Value: 1,
					},
					{
						Key:   "validFrom",
						Value: -1,
					},
					{
						Key:   "createdAt",
						Value: -1,
					},
				},
			),
	)

	if err != nil {
		return nil, nil, err
	}
	defer cursor.Close(ctx)

	allMemberships := make(
		[]models.CrewMembership,
		0,
	)

	if err := cursor.All(
		ctx,
		&allMemberships,
	); err != nil {
		return nil, nil, err
	}

	for _, membership := range allMemberships {
		result[membership.StudentID] = append(
			result[membership.StudentID],
			membership,
		)
	}

	return result, allMemberships, nil
}

func (
	handler *StudentHistoryHandler,
) loadMembershipCrews(
	ctx context.Context,
	memberships []models.CrewMembership,
) (map[bson.ObjectID]models.Crew, error) {
	result := make(
		map[bson.ObjectID]models.Crew,
	)

	if len(memberships) == 0 {
		return result, nil
	}

	crewIDs := make(
		[]bson.ObjectID,
		0,
		len(memberships),
	)

	seen := make(
		map[bson.ObjectID]struct{},
	)

	for _, membership := range memberships {
		if _, exists := seen[membership.CrewID]; exists {
			continue
		}

		seen[membership.CrewID] = struct{}{}

		crewIDs = append(
			crewIDs,
			membership.CrewID,
		)
	}

	cursor, err := handler.crews.Find(
		ctx,
		bson.M{
			"_id": bson.M{
				"$in": crewIDs,
			},
		},
	)

	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var crews []models.Crew

	if err := cursor.All(
		ctx,
		&crews,
	); err != nil {
		return nil, err
	}

	for _, crew := range crews {
		result[crew.ID] = crew
	}

	return result, nil
}

func validateStudentReactivation(
	student *models.User,
	crew *models.Crew,
	now time.Time,
) error {
	if student.Role != models.RoleStudent {
		return errors.New(
			"el usuario seleccionado no es un alumno",
		)
	}

	if student.Status == models.StatusBlocked {
		return errors.New(
			"un alumno bloqueado no puede ser reactivado",
		)
	}

	if crew.Status == models.CrewStatusFinished {
		return errors.New(
			"no se pueden incorporar alumnos a una cuadrilla finalizada",
		)
	}

	if crew.Status == models.CrewStatusCancelled {
		return errors.New(
			"no se pueden incorporar alumnos a una cuadrilla cancelada",
		)
	}

	if !crew.EndAt.After(now) {
		return errors.New(
			"la vigencia de la cuadrilla ya terminó",
		)
	}

	if crew.StudentLimit < 1 {
		return errors.New(
			"la cuadrilla no tiene un límite de alumnos válido",
		)
	}

	return nil
}

func studentHistoryObjectIDFromParam(
	c *gin.Context,
	paramName string,
	errorMessage string,
) (bson.ObjectID, bool) {
	value := strings.TrimSpace(
		c.Param(paramName),
	)

	objectID, err := bson.ObjectIDFromHex(value)

	if err != nil {
		c.JSON(
			http.StatusBadRequest,
			gin.H{
				"status":  "error",
				"message": errorMessage,
			},
		)

		return bson.NilObjectID, false
	}

	return objectID, true
}

func parseStudentHistoryPositiveInteger(
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
