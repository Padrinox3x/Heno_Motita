package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"heno-motita-api/internal/database"
	"heno-motita-api/internal/dto"
	"heno-motita-api/internal/middleware"
	"heno-motita-api/internal/models"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// CrewHandler contiene las colecciones utilizadas
// por el módulo de cuadrillas.
type CrewHandler struct {
	crews *mongo.Collection
	users *mongo.Collection
}

// NewCrewHandler crea el controlador de cuadrillas.
func NewCrewHandler(
	mongodb *database.MongoDB,
) *CrewHandler {
	return &CrewHandler{
		crews: mongodb.Collection("crews"),
		users: mongodb.Collection("users"),
	}
}

// Create procesa:
//
// POST /api/v1/crews
func (handler *CrewHandler) Create(
	c *gin.Context,
) {
	var request dto.CreateCrewRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(
			http.StatusBadRequest,
			gin.H{
				"status":  "error",
				"message": "Los datos de la cuadrilla no son válidos",
				"details": err.Error(),
			},
		)
		return
	}

	name := strings.TrimSpace(request.Name)
	description := strings.TrimSpace(request.Description)
	zone := strings.TrimSpace(request.Zone)
	institution := strings.TrimSpace(request.Institution)

	if err := validateCrewFields(
		name,
		zone,
		institution,
		request.StartAt,
		request.EndAt,
		request.StudentLimit,
	); err != nil {
		c.JSON(
			http.StatusBadRequest,
			gin.H{
				"status":  "error",
				"message": err.Error(),
			},
		)
		return
	}

	managerID, err := bson.ObjectIDFromHex(
		strings.TrimSpace(request.ManagerID),
	)
	if err != nil {
		c.JSON(
			http.StatusBadRequest,
			gin.H{
				"status":  "error",
				"message": "El identificador del encargado no es válido",
			},
		)
		return
	}

	ctx, cancel := context.WithTimeout(
		c.Request.Context(),
		10*time.Second,
	)
	defer cancel()

	manager, err := handler.findActiveManager(
		ctx,
		managerID,
	)
	if errors.Is(err, mongo.ErrNoDocuments) {
		c.JSON(
			http.StatusBadRequest,
			gin.H{
				"status":  "error",
				"message": "El encargado no existe, no está activo o no tiene el rol correcto",
			},
		)
		return
	}

	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "No fue posible validar al encargado",
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

	now := time.Now().UTC()
	startAt := request.StartAt.UTC()
	endAt := request.EndAt.UTC()

	crew := models.Crew{
		ID:           bson.NewObjectID(),
		Name:         name,
		Description:  description,
		Zone:         zone,
		Institution:  institution,
		ManagerID:    managerID,
		StartAt:      startAt,
		EndAt:        endAt,
		StudentLimit: request.StudentLimit,
		Status: determineCrewStatus(
			startAt,
			endAt,
			now,
		),
		CreatedBy: currentUser.ID,
		CreatedAt: now,
		UpdatedAt: now,
	}

	_, err = handler.crews.InsertOne(
		ctx,
		crew,
	)
	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "No fue posible registrar la cuadrilla",
			},
		)
		return
	}

	c.JSON(
		http.StatusCreated,
		gin.H{
			"message": "Cuadrilla registrada correctamente",
			"crew": dto.NewCrewResponse(
				crew,
				manager,
			),
		},
	)
}

// List procesa:
//
// GET /api/v1/crews
//
// Parámetros opcionales:
//
// ?page=1
// ?limit=10
// ?search=tula
// ?status=ACTIVE
// ?managerId=ObjectID
func (handler *CrewHandler) List(
	c *gin.Context,
) {
	page, err := parseCrewPositiveInteger(
		c.Query("page"),
		1,
	)
	if err != nil {
		c.JSON(
			http.StatusBadRequest,
			gin.H{
				"status":  "error",
				"message": "El parámetro page debe ser mayor que cero",
			},
		)
		return
	}

	limit, err := parseCrewPositiveInteger(
		c.Query("limit"),
		10,
	)
	if err != nil || limit > 100 {
		c.JSON(
			http.StatusBadRequest,
			gin.H{
				"status":  "error",
				"message": "El parámetro limit debe estar entre 1 y 100",
			},
		)
		return
	}

	filter := bson.M{}

	statusText := strings.ToUpper(
		strings.TrimSpace(
			c.Query("status"),
		),
	)

	if statusText != "" {
		status := models.CrewStatus(
			statusText,
		)

		if !isValidCrewStatus(status) {
			c.JSON(
				http.StatusBadRequest,
				gin.H{
					"status":  "error",
					"message": "El estado debe ser PENDING, ACTIVE, FINISHED o CANCELLED",
				},
			)
			return
		}

		filter["status"] = status
	}

	managerIDText := strings.TrimSpace(
		c.Query("managerId"),
	)

	if managerIDText != "" {
		managerID, err := bson.ObjectIDFromHex(
			managerIDText,
		)
		if err != nil {
			c.JSON(
				http.StatusBadRequest,
				gin.H{
					"status":  "error",
					"message": "El managerId no es válido",
				},
			)
			return
		}

		filter["managerId"] = managerID
	}

	searchText := strings.TrimSpace(
		c.Query("search"),
	)

	if searchText != "" {
		safeSearch := regexp.QuoteMeta(
			searchText,
		)

		filter["$or"] = bson.A{
			bson.M{
				"name": bson.M{
					"$regex":   safeSearch,
					"$options": "i",
				},
			},
			bson.M{
				"description": bson.M{
					"$regex":   safeSearch,
					"$options": "i",
				},
			},
			bson.M{
				"zone": bson.M{
					"$regex":   safeSearch,
					"$options": "i",
				},
			},
			bson.M{
				"institution": bson.M{
					"$regex":   safeSearch,
					"$options": "i",
				},
			},
		}
	}

	ctx, cancel := context.WithTimeout(
		c.Request.Context(),
		10*time.Second,
	)
	defer cancel()

	total, err := handler.crews.CountDocuments(
		ctx,
		filter,
	)
	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "No fue posible contar las cuadrillas",
			},
		)
		return
	}

	skip := int64(
		(page - 1) * limit,
	)

	findOptions := options.Find().
		SetSort(
			bson.D{
				{
					Key:   "createdAt",
					Value: -1,
				},
			},
		).
		SetSkip(skip).
		SetLimit(int64(limit))

	cursor, err := handler.crews.Find(
		ctx,
		filter,
		findOptions,
	)
	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "No fue posible consultar las cuadrillas",
			},
		)
		return
	}
	defer cursor.Close(ctx)

	crews := make(
		[]models.Crew,
		0,
	)

	if err := cursor.All(
		ctx,
		&crews,
	); err != nil {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "No fue posible procesar las cuadrillas",
			},
		)
		return
	}

	managerMap, err := handler.loadManagers(
		ctx,
		crews,
	)
	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "No fue posible recuperar los encargados",
			},
		)
		return
	}

	responses := make(
		[]dto.CrewResponse,
		0,
		len(crews),
	)

	for _, crew := range crews {
		var manager *models.User

		managerValue, exists := managerMap[crew.ManagerID]
		if exists {
			managerCopy := managerValue
			manager = &managerCopy
		}

		responses = append(
			responses,
			dto.NewCrewResponse(
				crew,
				manager,
			),
		)
	}

	totalPages := int64(0)

	if total > 0 {
		totalPages = (total + int64(limit) - 1) / int64(limit)
	}

	c.JSON(
		http.StatusOK,
		dto.CrewListResponse{
			Crews: responses,
			Pagination: dto.PaginationResponse{
				Page:       page,
				Limit:      limit,
				Total:      total,
				TotalPages: totalPages,
			},
		},
	)
}

// GetByID procesa:
//
// GET /api/v1/crews/:id
//
// La cuadrilla ya fue validada por RequireCrewAccess.
func (handler *CrewHandler) GetByID(
	c *gin.Context,
) {
	crew, ok := middleware.GetCurrentCrew(c)
	if !ok {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "No se pudo recuperar la cuadrilla validada",
			},
		)
		return
	}

	ctx, cancel := context.WithTimeout(
		c.Request.Context(),
		5*time.Second,
	)
	defer cancel()

	var manager models.User

	err := handler.users.FindOne(
		ctx,
		bson.M{
			"_id":  crew.ManagerID,
			"role": models.RoleManager,
		},
	).Decode(&manager)

	if errors.Is(err, mongo.ErrNoDocuments) {
		c.JSON(
			http.StatusOK,
			gin.H{
				"crew": dto.NewCrewResponse(
					*crew,
					nil,
				),
			},
		)
		return
	}

	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "No fue posible recuperar al encargado",
			},
		)
		return
	}

	c.JSON(
		http.StatusOK,
		gin.H{
			"crew": dto.NewCrewResponse(
				*crew,
				&manager,
			),
		},
	)
}

// Update procesa:
//
// PUT /api/v1/crews/:id
func (handler *CrewHandler) Update(
	c *gin.Context,
) {
	crewID, ok := crewIDFromParam(c)
	if !ok {
		return
	}

	var request dto.UpdateCrewRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(
			http.StatusBadRequest,
			gin.H{
				"status":  "error",
				"message": "Los datos de la cuadrilla no son válidos",
				"details": err.Error(),
			},
		)
		return
	}

	name := strings.TrimSpace(request.Name)
	description := strings.TrimSpace(request.Description)
	zone := strings.TrimSpace(request.Zone)
	institution := strings.TrimSpace(request.Institution)

	if err := validateCrewFields(
		name,
		zone,
		institution,
		request.StartAt,
		request.EndAt,
		request.StudentLimit,
	); err != nil {
		c.JSON(
			http.StatusBadRequest,
			gin.H{
				"status":  "error",
				"message": err.Error(),
			},
		)
		return
	}

	managerID, err := bson.ObjectIDFromHex(
		strings.TrimSpace(request.ManagerID),
	)
	if err != nil {
		c.JSON(
			http.StatusBadRequest,
			gin.H{
				"status":  "error",
				"message": "El identificador del encargado no es válido",
			},
		)
		return
	}

	ctx, cancel := context.WithTimeout(
		c.Request.Context(),
		10*time.Second,
	)
	defer cancel()

	var currentCrew models.Crew

	err = handler.crews.FindOne(
		ctx,
		bson.M{
			"_id": crewID,
		},
	).Decode(&currentCrew)

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

	if currentCrew.Status == models.CrewStatusFinished ||
		currentCrew.Status == models.CrewStatusCancelled {
		c.JSON(
			http.StatusConflict,
			gin.H{
				"status":  "error",
				"message": "Una cuadrilla finalizada o cancelada no puede modificarse",
			},
		)
		return
	}

	manager, err := handler.findActiveManager(
		ctx,
		managerID,
	)
	if errors.Is(err, mongo.ErrNoDocuments) {
		c.JSON(
			http.StatusBadRequest,
			gin.H{
				"status":  "error",
				"message": "El encargado no existe, no está activo o no tiene el rol correcto",
			},
		)
		return
	}

	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "No fue posible validar al encargado",
			},
		)
		return
	}

	startAt := request.StartAt.UTC()
	endAt := request.EndAt.UTC()
	now := time.Now().UTC()

	update := bson.M{
		"$set": bson.M{
			"name":         name,
			"description":  description,
			"zone":         zone,
			"institution":  institution,
			"managerId":    managerID,
			"startAt":      startAt,
			"endAt":        endAt,
			"studentLimit": request.StudentLimit,
			"status": determineCrewStatus(
				startAt,
				endAt,
				now,
			),
			"updatedAt": now,
		},
	}

	result, err := handler.crews.UpdateOne(
		ctx,
		bson.M{
			"_id": crewID,
		},
		update,
	)
	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "No fue posible actualizar la cuadrilla",
			},
		)
		return
	}

	if result.MatchedCount == 0 {
		c.JSON(
			http.StatusNotFound,
			gin.H{
				"status":  "error",
				"message": "La cuadrilla no existe",
			},
		)
		return
	}

	var updatedCrew models.Crew

	err = handler.crews.FindOne(
		ctx,
		bson.M{
			"_id": crewID,
		},
	).Decode(&updatedCrew)
	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "La cuadrilla se actualizó, pero no pudo recuperarse",
			},
		)
		return
	}

	c.JSON(
		http.StatusOK,
		gin.H{
			"message": "Cuadrilla actualizada correctamente",
			"crew": dto.NewCrewResponse(
				updatedCrew,
				manager,
			),
		},
	)
}

// UpdateStatus procesa:
//
// PATCH /api/v1/crews/:id/status
func (handler *CrewHandler) UpdateStatus(
	c *gin.Context,
) {
	crewID, ok := crewIDFromParam(c)
	if !ok {
		return
	}

	var request dto.UpdateCrewStatusRequest

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

	newStatus := models.CrewStatus(
		strings.ToUpper(
			strings.TrimSpace(
				string(request.Status),
			),
		),
	)

	if !isValidCrewStatus(newStatus) {
		c.JSON(
			http.StatusBadRequest,
			gin.H{
				"status":  "error",
				"message": "El estado debe ser PENDING, ACTIVE, FINISHED o CANCELLED",
			},
		)
		return
	}

	ctx, cancel := context.WithTimeout(
		c.Request.Context(),
		10*time.Second,
	)
	defer cancel()

	var currentCrew models.Crew

	err := handler.crews.FindOne(
		ctx,
		bson.M{
			"_id": crewID,
		},
	).Decode(&currentCrew)

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

	if currentCrew.Status == models.CrewStatusFinished ||
		currentCrew.Status == models.CrewStatusCancelled {
		if newStatus != currentCrew.Status {
			c.JSON(
				http.StatusConflict,
				gin.H{
					"status":  "error",
					"message": "Una cuadrilla finalizada o cancelada no puede reactivarse",
				},
			)
			return
		}
	}

	now := time.Now().UTC()

	if err := validateRequestedCrewStatus(
		newStatus,
		currentCrew.StartAt,
		currentCrew.EndAt,
		now,
	); err != nil {
		c.JSON(
			http.StatusBadRequest,
			gin.H{
				"status":  "error",
				"message": err.Error(),
			},
		)
		return
	}

	fields := bson.M{
		"status":    newStatus,
		"updatedAt": now,
	}

	switch newStatus {
	case models.CrewStatusFinished:
		fields["finishedAt"] = now

	case models.CrewStatusCancelled:
		fields["cancelledAt"] = now
	}

	result, err := handler.crews.UpdateOne(
		ctx,
		bson.M{
			"_id": crewID,
		},
		bson.M{
			"$set": fields,
		},
	)
	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "No fue posible cambiar el estado de la cuadrilla",
			},
		)
		return
	}

	if result.MatchedCount == 0 {
		c.JSON(
			http.StatusNotFound,
			gin.H{
				"status":  "error",
				"message": "La cuadrilla no existe",
			},
		)
		return
	}

	var updatedCrew models.Crew

	err = handler.crews.FindOne(
		ctx,
		bson.M{
			"_id": crewID,
		},
	).Decode(&updatedCrew)
	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "El estado cambió, pero no pudo recuperarse la cuadrilla",
			},
		)
		return
	}

	var manager models.User
	managerPointer := (*models.User)(nil)

	err = handler.users.FindOne(
		ctx,
		bson.M{
			"_id":  updatedCrew.ManagerID,
			"role": models.RoleManager,
		},
	).Decode(&manager)

	if err == nil {
		managerPointer = &manager
	}

	c.JSON(
		http.StatusOK,
		gin.H{
			"message": "Estado de la cuadrilla actualizado correctamente",
			"crew": dto.NewCrewResponse(
				updatedCrew,
				managerPointer,
			),
		},
	)
}

// findActiveManager valida que el usuario exista,
// sea encargado y continúe activo.
func (handler *CrewHandler) findActiveManager(
	ctx context.Context,
	managerID bson.ObjectID,
) (*models.User, error) {
	var manager models.User

	err := handler.users.FindOne(
		ctx,
		bson.M{
			"_id":    managerID,
			"role":   models.RoleManager,
			"status": models.StatusActive,
		},
	).Decode(&manager)

	if err != nil {
		return nil, err
	}

	return &manager, nil
}

// loadManagers recupera todos los encargados requeridos
// por una lista de cuadrillas usando una sola consulta.
func (handler *CrewHandler) loadManagers(
	ctx context.Context,
	crews []models.Crew,
) (map[bson.ObjectID]models.User, error) {
	managerMap := make(
		map[bson.ObjectID]models.User,
	)

	if len(crews) == 0 {
		return managerMap, nil
	}

	managerIDs := make(
		[]bson.ObjectID,
		0,
		len(crews),
	)

	seen := make(
		map[bson.ObjectID]struct{},
	)

	for _, crew := range crews {
		if _, exists := seen[crew.ManagerID]; exists {
			continue
		}

		seen[crew.ManagerID] = struct{}{}

		managerIDs = append(
			managerIDs,
			crew.ManagerID,
		)
	}

	cursor, err := handler.users.Find(
		ctx,
		bson.M{
			"_id": bson.M{
				"$in": managerIDs,
			},
			"role": models.RoleManager,
		},
	)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var managers []models.User

	if err := cursor.All(
		ctx,
		&managers,
	); err != nil {
		return nil, err
	}

	for _, manager := range managers {
		managerMap[manager.ID] = manager
	}

	return managerMap, nil
}

func crewIDFromParam(
	c *gin.Context,
) (bson.ObjectID, bool) {
	idText := strings.TrimSpace(
		c.Param("id"),
	)

	crewID, err := bson.ObjectIDFromHex(
		idText,
	)
	if err != nil {
		c.JSON(
			http.StatusBadRequest,
			gin.H{
				"status":  "error",
				"message": "El identificador de la cuadrilla no es válido",
			},
		)

		return bson.ObjectID{}, false
	}

	return crewID, true
}

func validateCrewFields(
	name string,
	zone string,
	institution string,
	startAt time.Time,
	endAt time.Time,
	studentLimit int,
) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf(
			"el nombre de la cuadrilla es obligatorio",
		)
	}

	if strings.TrimSpace(zone) == "" {
		return fmt.Errorf(
			"la zona es obligatoria",
		)
	}

	if strings.TrimSpace(institution) == "" {
		return fmt.Errorf(
			"la institución es obligatoria",
		)
	}

	if startAt.IsZero() {
		return fmt.Errorf(
			"la fecha de inicio es obligatoria",
		)
	}

	if endAt.IsZero() {
		return fmt.Errorf(
			"la fecha final es obligatoria",
		)
	}

	if !endAt.After(startAt) {
		return fmt.Errorf(
			"la fecha final debe ser posterior a la fecha de inicio",
		)
	}

	if !endAt.After(time.Now().UTC()) {
		return fmt.Errorf(
			"la fecha final debe estar en el futuro",
		)
	}

	if studentLimit < 1 || studentLimit > 100 {
		return fmt.Errorf(
			"el límite de alumnos debe estar entre 1 y 100",
		)
	}

	return nil
}

func determineCrewStatus(
	startAt time.Time,
	endAt time.Time,
	now time.Time,
) models.CrewStatus {
	if now.Before(startAt) {
		return models.CrewStatusPending
	}

	if now.Before(endAt) {
		return models.CrewStatusActive
	}

	return models.CrewStatusFinished
}

func validateRequestedCrewStatus(
	status models.CrewStatus,
	startAt time.Time,
	endAt time.Time,
	now time.Time,
) error {
	switch status {
	case models.CrewStatusPending:
		if !now.Before(startAt) {
			return fmt.Errorf(
				"la cuadrilla solo puede quedar PENDING antes de su fecha de inicio",
			)
		}

	case models.CrewStatusActive:
		if now.Before(startAt) {
			return fmt.Errorf(
				"la cuadrilla no puede activarse antes de su fecha de inicio",
			)
		}

		if !now.Before(endAt) {
			return fmt.Errorf(
				"la cuadrilla no puede activarse después de su fecha final",
			)
		}

	case models.CrewStatusFinished:
		return nil

	case models.CrewStatusCancelled:
		return nil

	default:
		return fmt.Errorf(
			"el estado de la cuadrilla no es válido",
		)
	}

	return nil
}

func isValidCrewStatus(
	status models.CrewStatus,
) bool {
	switch status {
	case models.CrewStatusPending,
		models.CrewStatusActive,
		models.CrewStatusFinished,
		models.CrewStatusCancelled:
		return true

	default:
		return false
	}
}

func parseCrewPositiveInteger(
	value string,
	defaultValue int,
) (int, error) {
	value = strings.TrimSpace(value)

	if value == "" {
		return defaultValue, nil
	}

	number, err := strconv.Atoi(value)
	if err != nil || number < 1 {
		return 0, fmt.Errorf(
			"el valor debe ser un entero mayor que cero",
		)
	}

	return number, nil
}
