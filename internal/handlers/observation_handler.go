package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
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

// ObservationHandler contiene las colecciones utilizadas
// por el módulo de observaciones.
type ObservationHandler struct {
	observations *mongo.Collection
	users        *mongo.Collection
}

func NewObservationHandler(
	mongodb *database.MongoDB,
) *ObservationHandler {
	return &ObservationHandler{
		observations: mongodb.Collection("observations"),
		users:        mongodb.Collection("users"),
	}
}

// Create procesa:
//
// POST /api/v1/trees/:id/observations
func (handler *ObservationHandler) Create(
	c *gin.Context,
) {
	tree, ok := middleware.GetCurrentTree(c)
	if !ok {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "No fue posible recuperar el árbol",
			},
		)
		return
	}

	crew, ok := middleware.GetCurrentCrew(c)
	if !ok {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "No fue posible recuperar la cuadrilla",
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

	var request dto.CreateObservationRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(
			http.StatusBadRequest,
			gin.H{
				"status":  "error",
				"message": "Los datos de la observación no son válidos",
				"details": err.Error(),
			},
		)
		return
	}

	lowerScore, middleScore, upperScore, totalScore, err :=
		validateAndCalculateHawksworth(
			request.LowerThirdScore,
			request.MiddleThirdScore,
			request.UpperThirdScore,
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

	observationDate := request.ObservationDate.UTC()
	now := time.Now().UTC()

	if err := validateObservationDate(
		observationDate,
		crew,
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

	latitude, longitude, err := validateObservationCoordinates(
		request.Latitude,
		request.Longitude,
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

	if err := validateObservationOperation(
		tree,
		crew,
		currentUser,
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

	observation := models.Observation{
		ID:               bson.NewObjectID(),
		TreeID:           tree.ID,
		CrewID:           crew.ID,
		ObserverID:       currentUser.ID,
		Method:           models.AssessmentMethodHawksworth,
		LowerThirdScore:  lowerScore,
		MiddleThirdScore: middleScore,
		UpperThirdScore:  upperScore,
		TotalScore:       totalScore,
		Notes:            strings.TrimSpace(request.Notes),
		ObservationDate:  observationDate,
		Latitude:         latitude,
		Longitude:        longitude,
		Status:           models.ObservationStatusActive,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	ctx, cancel := context.WithTimeout(
		c.Request.Context(),
		10*time.Second,
	)
	defer cancel()

	_, err = handler.observations.InsertOne(
		ctx,
		observation,
	)
	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "No fue posible registrar la observación",
			},
		)
		return
	}

	c.JSON(
		http.StatusCreated,
		gin.H{
			"message": "Observación registrada correctamente",
			"observation": dto.NewObservationResponse(
				observation,
				tree,
				currentUser,
			),
		},
	)
}

// ListByTree procesa:
//
// GET /api/v1/trees/:id/observations
//
// Parámetros:
//
// ?page=1
// ?limit=10
// ?status=ACTIVE
func (handler *ObservationHandler) ListByTree(
	c *gin.Context,
) {
	tree, ok := middleware.GetCurrentTree(c)
	if !ok {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "No fue posible recuperar el árbol",
			},
		)
		return
	}

	page, valid := parseObservationPositiveInteger(
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

	limit, valid := parseObservationPositiveInteger(
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
		"treeId": tree.ID,
	}

	statusText := strings.ToUpper(
		strings.TrimSpace(
			c.Query("status"),
		),
	)

	if statusText != "" {
		status := models.ObservationStatus(
			statusText,
		)

		if !isValidObservationStatus(status) {
			c.JSON(
				http.StatusBadRequest,
				gin.H{
					"status":  "error",
					"message": "El estado debe ser ACTIVE o ARCHIVED",
				},
			)
			return
		}

		filter["status"] = status
	}

	ctx, cancel := context.WithTimeout(
		c.Request.Context(),
		12*time.Second,
	)
	defer cancel()

	total, err := handler.observations.CountDocuments(
		ctx,
		filter,
	)
	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "No fue posible contar las observaciones",
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
					Key:   "observationDate",
					Value: -1,
				},
			},
		).
		SetSkip(skip).
		SetLimit(int64(limit))

	cursor, err := handler.observations.Find(
		ctx,
		filter,
		findOptions,
	)
	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "No fue posible consultar las observaciones",
			},
		)
		return
	}
	defer cursor.Close(ctx)

	observations := make(
		[]models.Observation,
		0,
	)

	if err := cursor.All(
		ctx,
		&observations,
	); err != nil {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "No fue posible procesar las observaciones",
			},
		)
		return
	}

	observers, err := handler.loadObservationUsers(
		ctx,
		observations,
	)
	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "No fue posible recuperar los observadores",
			},
		)
		return
	}

	responses := make(
		[]dto.ObservationResponse,
		0,
		len(observations),
	)

	for _, observation := range observations {
		var observerPointer *models.User

		observer, exists := observers[observation.ObserverID]
		if exists {
			observerCopy := observer
			observerPointer = &observerCopy
		}

		responses = append(
			responses,
			dto.NewObservationResponse(
				observation,
				tree,
				observerPointer,
			),
		)
	}

	totalPages := int64(0)

	if total > 0 {
		totalPages = (total + int64(limit) - 1) / int64(limit)
	}

	c.JSON(
		http.StatusOK,
		dto.ObservationListResponse{
			Observations: responses,
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
// GET /api/v1/observations/:id
func (handler *ObservationHandler) GetByID(
	c *gin.Context,
) {
	observation, ok := middleware.GetCurrentObservation(c)
	if !ok {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "No fue posible recuperar la observación",
			},
		)
		return
	}

	tree, ok := middleware.GetCurrentTree(c)
	if !ok {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "No fue posible recuperar el árbol",
			},
		)
		return
	}

	ctx, cancel := context.WithTimeout(
		c.Request.Context(),
		5*time.Second,
	)
	defer cancel()

	observer, err := handler.findObservationUser(
		ctx,
		observation.ObserverID,
	)

	if errors.Is(err, mongo.ErrNoDocuments) {
		observer = nil
	} else if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "No fue posible recuperar al observador",
			},
		)
		return
	}

	c.JSON(
		http.StatusOK,
		gin.H{
			"observation": dto.NewObservationResponse(
				*observation,
				tree,
				observer,
			),
		},
	)
}

// Update procesa:
//
// PUT /api/v1/observations/:id
func (handler *ObservationHandler) Update(
	c *gin.Context,
) {
	observation, ok := middleware.GetCurrentObservation(c)
	if !ok {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "No fue posible recuperar la observación",
			},
		)
		return
	}

	tree, ok := middleware.GetCurrentTree(c)
	if !ok {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "No fue posible recuperar el árbol",
			},
		)
		return
	}

	crew, ok := middleware.GetCurrentCrew(c)
	if !ok {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "No fue posible recuperar la cuadrilla",
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

	if observation.Status == models.ObservationStatusArchived {
		c.JSON(
			http.StatusConflict,
			gin.H{
				"status":  "error",
				"message": "Una observación archivada no puede modificarse",
			},
		)
		return
	}

	var request dto.UpdateObservationRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(
			http.StatusBadRequest,
			gin.H{
				"status":  "error",
				"message": "Los datos de la observación no son válidos",
				"details": err.Error(),
			},
		)
		return
	}

	lowerScore, middleScore, upperScore, totalScore, err :=
		validateAndCalculateHawksworth(
			request.LowerThirdScore,
			request.MiddleThirdScore,
			request.UpperThirdScore,
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

	now := time.Now().UTC()
	observationDate := request.ObservationDate.UTC()

	if err := validateObservationDate(
		observationDate,
		crew,
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

	latitude, longitude, err := validateObservationCoordinates(
		request.Latitude,
		request.Longitude,
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

	if err := validateObservationOperation(
		tree,
		crew,
		currentUser,
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

	ctx, cancel := context.WithTimeout(
		c.Request.Context(),
		10*time.Second,
	)
	defer cancel()

	result, err := handler.observations.UpdateOne(
		ctx,
		bson.M{
			"_id": observation.ID,
		},
		bson.M{
			"$set": bson.M{
				"lowerThirdScore":  lowerScore,
				"middleThirdScore": middleScore,
				"upperThirdScore":  upperScore,
				"totalScore":       totalScore,
				"notes":            strings.TrimSpace(request.Notes),
				"observationDate":  observationDate,
				"latitude":         latitude,
				"longitude":        longitude,
				"updatedAt":        now,
			},
		},
	)
	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "No fue posible actualizar la observación",
			},
		)
		return
	}

	if result.MatchedCount == 0 {
		c.JSON(
			http.StatusNotFound,
			gin.H{
				"status":  "error",
				"message": "La observación no existe",
			},
		)
		return
	}

	var updatedObservation models.Observation

	err = handler.observations.FindOne(
		ctx,
		bson.M{
			"_id": observation.ID,
		},
	).Decode(&updatedObservation)
	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "La observación se actualizó, pero no pudo recuperarse",
			},
		)
		return
	}

	observer, err := handler.findObservationUser(
		ctx,
		updatedObservation.ObserverID,
	)

	if errors.Is(err, mongo.ErrNoDocuments) {
		observer = nil
	} else if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "No fue posible recuperar al observador",
			},
		)
		return
	}

	c.JSON(
		http.StatusOK,
		gin.H{
			"message": "Observación actualizada correctamente",
			"observation": dto.NewObservationResponse(
				updatedObservation,
				tree,
				observer,
			),
		},
	)
}

// UpdateStatus procesa:
//
// PATCH /api/v1/observations/:id/status
func (handler *ObservationHandler) UpdateStatus(
	c *gin.Context,
) {
	observation, ok := middleware.GetCurrentObservation(c)
	if !ok {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "No fue posible recuperar la observación",
			},
		)
		return
	}

	tree, ok := middleware.GetCurrentTree(c)
	if !ok {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "No fue posible recuperar el árbol",
			},
		)
		return
	}

	var request dto.UpdateObservationStatusRequest

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

	newStatus := models.ObservationStatus(
		strings.ToUpper(
			strings.TrimSpace(
				string(request.Status),
			),
		),
	)

	if !isValidObservationStatus(newStatus) {
		c.JSON(
			http.StatusBadRequest,
			gin.H{
				"status":  "error",
				"message": "El estado debe ser ACTIVE o ARCHIVED",
			},
		)
		return
	}

	if observation.Status == models.ObservationStatusArchived &&
		newStatus != models.ObservationStatusArchived {
		c.JSON(
			http.StatusConflict,
			gin.H{
				"status":  "error",
				"message": "Una observación archivada no puede reactivarse",
			},
		)
		return
	}

	now := time.Now().UTC()

	fields := bson.M{
		"status":    newStatus,
		"updatedAt": now,
	}

	if newStatus == models.ObservationStatusArchived {
		fields["archivedAt"] = now
	}

	ctx, cancel := context.WithTimeout(
		c.Request.Context(),
		10*time.Second,
	)
	defer cancel()

	result, err := handler.observations.UpdateOne(
		ctx,
		bson.M{
			"_id": observation.ID,
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
				"message": "No fue posible cambiar el estado",
			},
		)
		return
	}

	if result.MatchedCount == 0 {
		c.JSON(
			http.StatusNotFound,
			gin.H{
				"status":  "error",
				"message": "La observación no existe",
			},
		)
		return
	}

	var updatedObservation models.Observation

	err = handler.observations.FindOne(
		ctx,
		bson.M{
			"_id": observation.ID,
		},
	).Decode(&updatedObservation)
	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "El estado cambió, pero la observación no pudo recuperarse",
			},
		)
		return
	}

	observer, err := handler.findObservationUser(
		ctx,
		updatedObservation.ObserverID,
	)

	if errors.Is(err, mongo.ErrNoDocuments) {
		observer = nil
	} else if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "No fue posible recuperar al observador",
			},
		)
		return
	}

	c.JSON(
		http.StatusOK,
		gin.H{
			"message": "Estado de la observación actualizado correctamente",
			"observation": dto.NewObservationResponse(
				updatedObservation,
				tree,
				observer,
			),
		},
	)
}

func (handler *ObservationHandler) findObservationUser(
	ctx context.Context,
	userID bson.ObjectID,
) (*models.User, error) {
	var user models.User

	err := handler.users.FindOne(
		ctx,
		bson.M{
			"_id": userID,
		},
	).Decode(&user)

	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (handler *ObservationHandler) loadObservationUsers(
	ctx context.Context,
	observations []models.Observation,
) (map[bson.ObjectID]models.User, error) {
	result := make(
		map[bson.ObjectID]models.User,
	)

	if len(observations) == 0 {
		return result, nil
	}

	userIDs := make(
		[]bson.ObjectID,
		0,
		len(observations),
	)

	seen := make(
		map[bson.ObjectID]struct{},
	)

	for _, observation := range observations {
		if _, exists := seen[observation.ObserverID]; exists {
			continue
		}

		seen[observation.ObserverID] = struct{}{}

		userIDs = append(
			userIDs,
			observation.ObserverID,
		)
	}

	cursor, err := handler.users.Find(
		ctx,
		bson.M{
			"_id": bson.M{
				"$in": userIDs,
			},
		},
	)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var users []models.User

	if err := cursor.All(
		ctx,
		&users,
	); err != nil {
		return nil, err
	}

	for _, user := range users {
		result[user.ID] = user
	}

	return result, nil
}

// validateAndCalculateHawksworth valida los tres tercios
// y calcula el total.
//
// Cada tercio puede tener:
// 0, 1 o 2.
//
// Total:
// 0 a 6.
func validateAndCalculateHawksworth(
	lower *int,
	middle *int,
	upper *int,
) (int, int, int, int, error) {
	if lower == nil ||
		middle == nil ||
		upper == nil {
		return 0, 0, 0, 0, fmt.Errorf(
			"debes proporcionar la puntuación de los tres tercios",
		)
	}

	if *lower < 0 || *lower > 2 {
		return 0, 0, 0, 0, fmt.Errorf(
			"lowerThirdScore debe estar entre 0 y 2",
		)
	}

	if *middle < 0 || *middle > 2 {
		return 0, 0, 0, 0, fmt.Errorf(
			"middleThirdScore debe estar entre 0 y 2",
		)
	}

	if *upper < 0 || *upper > 2 {
		return 0, 0, 0, 0, fmt.Errorf(
			"upperThirdScore debe estar entre 0 y 2",
		)
	}

	total := *lower + *middle + *upper

	return *lower, *middle, *upper, total, nil
}

func validateObservationCoordinates(
	latitude *float64,
	longitude *float64,
) (*float64, *float64, error) {
	if latitude == nil && longitude == nil {
		return nil, nil, nil
	}

	if latitude == nil || longitude == nil {
		return nil, nil, fmt.Errorf(
			"debes enviar latitud y longitud juntas",
		)
	}

	if *latitude < -90 || *latitude > 90 {
		return nil, nil, fmt.Errorf(
			"la latitud debe estar entre -90 y 90",
		)
	}

	if *longitude < -180 || *longitude > 180 {
		return nil, nil, fmt.Errorf(
			"la longitud debe estar entre -180 y 180",
		)
	}

	latitudeCopy := *latitude
	longitudeCopy := *longitude

	return &latitudeCopy, &longitudeCopy, nil
}

func validateObservationDate(
	observationDate time.Time,
	crew *models.Crew,
	now time.Time,
) error {
	if observationDate.IsZero() {
		return fmt.Errorf(
			"la fecha de observación es obligatoria",
		)
	}

	if observationDate.Before(crew.StartAt) {
		return fmt.Errorf(
			"la observación no puede ser anterior al inicio de la cuadrilla",
		)
	}

	if observationDate.After(crew.EndAt) {
		return fmt.Errorf(
			"la observación no puede ser posterior al final de la cuadrilla",
		)
	}

	// Se permiten cinco minutos por posibles diferencias
	// entre el reloj del dispositivo y el servidor.
	if observationDate.After(
		now.Add(5 * time.Minute),
	) {
		return fmt.Errorf(
			"la fecha de observación no puede estar en el futuro",
		)
	}

	return nil
}

func validateObservationOperation(
	tree *models.Tree,
	crew *models.Crew,
	currentUser *models.User,
	now time.Time,
) error {
	if tree.Status != models.TreeStatusActive {
		return fmt.Errorf(
			"solo se pueden registrar observaciones en árboles activos",
		)
	}

	if crew.Status == models.CrewStatusFinished {
		return fmt.Errorf(
			"la cuadrilla ya finalizó",
		)
	}

	if crew.Status == models.CrewStatusCancelled {
		return fmt.Errorf(
			"la cuadrilla está cancelada",
		)
	}

	if !crew.EndAt.After(now) {
		return fmt.Errorf(
			"la vigencia de la cuadrilla ya terminó",
		)
	}

	if currentUser.Role == models.RoleStudent &&
		now.Before(crew.StartAt) {
		return fmt.Errorf(
			"la cuadrilla todavía no inicia actividades",
		)
	}

	return nil
}

func isValidObservationStatus(
	status models.ObservationStatus,
) bool {
	switch status {
	case models.ObservationStatusActive,
		models.ObservationStatusArchived:
		return true

	default:
		return false
	}
}

func parseObservationPositiveInteger(
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
