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

var treeCodePattern = regexp.MustCompile(
	`^[A-Z0-9][A-Z0-9-]{2,39}$`,
)

// TreeHandler contiene las colecciones utilizadas
// por el módulo de árboles.
type TreeHandler struct {
	trees *mongo.Collection
	users *mongo.Collection
}

func NewTreeHandler(
	mongodb *database.MongoDB,
) *TreeHandler {
	return &TreeHandler{
		trees: mongodb.Collection("trees"),
		users: mongodb.Collection("users"),
	}
}

// Create procesa:
//
// POST /api/v1/crews/:crewId/trees
func (handler *TreeHandler) Create(
	c *gin.Context,
) {
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

	var request dto.CreateTreeRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(
			http.StatusBadRequest,
			gin.H{
				"status":  "error",
				"message": "Los datos del árbol no son válidos",
				"details": err.Error(),
			},
		)
		return
	}

	code := normalizeTreeCode(request.Code)
	commonName := strings.TrimSpace(request.CommonName)
	scientificName := strings.TrimSpace(
		request.ScientificName,
	)
	locationDescription := strings.TrimSpace(
		request.LocationDescription,
	)

	if request.Latitude == nil ||
		request.Longitude == nil {
		c.JSON(
			http.StatusBadRequest,
			gin.H{
				"status":  "error",
				"message": "La latitud y longitud son obligatorias",
			},
		)
		return
	}

	latitude := *request.Latitude
	longitude := *request.Longitude

	if err := validateTreeFields(
		code,
		commonName,
		scientificName,
		latitude,
		longitude,
		locationDescription,
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

	now := time.Now().UTC()

	if err := validateCrewAllowsTreeChanges(
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

	tree := models.Tree{
		ID:                  bson.NewObjectID(),
		CrewID:              crew.ID,
		Code:                code,
		CommonName:          commonName,
		ScientificName:      scientificName,
		Latitude:            latitude,
		Longitude:           longitude,
		LocationDescription: locationDescription,
		Status:              models.TreeStatusActive,
		RegisteredBy:        currentUser.ID,
		CreatedAt:           now,
		UpdatedAt:           now,
	}

	ctx, cancel := context.WithTimeout(
		c.Request.Context(),
		10*time.Second,
	)
	defer cancel()

	_, err := handler.trees.InsertOne(
		ctx,
		tree,
	)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			c.JSON(
				http.StatusConflict,
				gin.H{
					"status":  "error",
					"message": "Ya existe un árbol con ese código",
				},
			)
			return
		}

		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "No fue posible registrar el árbol",
			},
		)
		return
	}

	c.JSON(
		http.StatusCreated,
		gin.H{
			"message": "Árbol registrado correctamente",
			"tree": dto.NewTreeResponse(
				tree,
				currentUser,
			),
		},
	)
}

// ListByCrew procesa:
//
// GET /api/v1/crews/:crewId/trees
//
// Parámetros opcionales:
//
// ?page=1
// ?limit=10
// ?search=mezquite
// ?status=ACTIVE
func (handler *TreeHandler) ListByCrew(
	c *gin.Context,
) {
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

	page, valid := parseTreePositiveInteger(
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

	limit, valid := parseTreePositiveInteger(
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
		"crewId": crew.ID,
	}

	statusText := strings.ToUpper(
		strings.TrimSpace(
			c.Query("status"),
		),
	)

	if statusText != "" {
		status := models.TreeStatus(statusText)

		if !isValidTreeStatus(status) {
			c.JSON(
				http.StatusBadRequest,
				gin.H{
					"status":  "error",
					"message": "El estado debe ser ACTIVE, INACTIVE o ARCHIVED",
				},
			)
			return
		}

		filter["status"] = status
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
				"code": bson.M{
					"$regex":   safeSearch,
					"$options": "i",
				},
			},
			bson.M{
				"commonName": bson.M{
					"$regex":   safeSearch,
					"$options": "i",
				},
			},
			bson.M{
				"scientificName": bson.M{
					"$regex":   safeSearch,
					"$options": "i",
				},
			},
			bson.M{
				"locationDescription": bson.M{
					"$regex":   safeSearch,
					"$options": "i",
				},
			},
		}
	}

	ctx, cancel := context.WithTimeout(
		c.Request.Context(),
		12*time.Second,
	)
	defer cancel()

	total, err := handler.trees.CountDocuments(
		ctx,
		filter,
	)
	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "No fue posible contar los árboles",
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

	cursor, err := handler.trees.Find(
		ctx,
		filter,
		findOptions,
	)
	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "No fue posible consultar los árboles",
			},
		)
		return
	}
	defer cursor.Close(ctx)

	trees := make(
		[]models.Tree,
		0,
	)

	if err := cursor.All(
		ctx,
		&trees,
	); err != nil {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "No fue posible procesar los árboles",
			},
		)
		return
	}

	usersMap, err := handler.loadTreeRegisteredUsers(
		ctx,
		trees,
	)
	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "No fue posible recuperar los usuarios de registro",
			},
		)
		return
	}

	responses := make(
		[]dto.TreeResponse,
		0,
		len(trees),
	)

	for _, tree := range trees {
		var registeredBy *models.User

		user, exists := usersMap[tree.RegisteredBy]
		if exists {
			userCopy := user
			registeredBy = &userCopy
		}

		responses = append(
			responses,
			dto.NewTreeResponse(
				tree,
				registeredBy,
			),
		)
	}

	totalPages := int64(0)

	if total > 0 {
		totalPages = (total + int64(limit) - 1) / int64(limit)
	}

	c.JSON(
		http.StatusOK,
		dto.TreeListResponse{
			Trees: responses,
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
// GET /api/v1/trees/:id
func (handler *TreeHandler) GetByID(
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

	ctx, cancel := context.WithTimeout(
		c.Request.Context(),
		5*time.Second,
	)
	defer cancel()

	registeredBy, err := handler.findTreeRegisteredUser(
		ctx,
		tree.RegisteredBy,
	)

	if errors.Is(err, mongo.ErrNoDocuments) {
		c.JSON(
			http.StatusOK,
			gin.H{
				"tree": dto.NewTreeResponse(
					*tree,
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
				"message": "No fue posible recuperar al usuario de registro",
			},
		)
		return
	}

	c.JSON(
		http.StatusOK,
		gin.H{
			"tree": dto.NewTreeResponse(
				*tree,
				registeredBy,
			),
		},
	)
}

// Update procesa:
//
// PUT /api/v1/trees/:id
func (handler *TreeHandler) Update(
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

	if tree.Status == models.TreeStatusArchived {
		c.JSON(
			http.StatusConflict,
			gin.H{
				"status":  "error",
				"message": "Un árbol archivado no puede modificarse",
			},
		)
		return
	}

	var request dto.UpdateTreeRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(
			http.StatusBadRequest,
			gin.H{
				"status":  "error",
				"message": "Los datos del árbol no son válidos",
				"details": err.Error(),
			},
		)
		return
	}

	if request.Latitude == nil ||
		request.Longitude == nil {
		c.JSON(
			http.StatusBadRequest,
			gin.H{
				"status":  "error",
				"message": "La latitud y longitud son obligatorias",
			},
		)
		return
	}

	code := normalizeTreeCode(request.Code)
	commonName := strings.TrimSpace(request.CommonName)
	scientificName := strings.TrimSpace(
		request.ScientificName,
	)
	locationDescription := strings.TrimSpace(
		request.LocationDescription,
	)
	latitude := *request.Latitude
	longitude := *request.Longitude

	if err := validateTreeFields(
		code,
		commonName,
		scientificName,
		latitude,
		longitude,
		locationDescription,
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

	now := time.Now().UTC()

	if err := validateCrewAllowsTreeChanges(
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

	result, err := handler.trees.UpdateOne(
		ctx,
		bson.M{
			"_id": tree.ID,
		},
		bson.M{
			"$set": bson.M{
				"code":                code,
				"commonName":          commonName,
				"scientificName":      scientificName,
				"latitude":            latitude,
				"longitude":           longitude,
				"locationDescription": locationDescription,
				"updatedAt":           now,
			},
		},
	)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			c.JSON(
				http.StatusConflict,
				gin.H{
					"status":  "error",
					"message": "Ya existe un árbol con ese código",
				},
			)
			return
		}

		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "No fue posible actualizar el árbol",
			},
		)
		return
	}

	if result.MatchedCount == 0 {
		c.JSON(
			http.StatusNotFound,
			gin.H{
				"status":  "error",
				"message": "El árbol no existe",
			},
		)
		return
	}

	var updatedTree models.Tree

	err = handler.trees.FindOne(
		ctx,
		bson.M{
			"_id": tree.ID,
		},
	).Decode(&updatedTree)
	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "El árbol se actualizó, pero no pudo recuperarse",
			},
		)
		return
	}

	registeredBy, err := handler.findTreeRegisteredUser(
		ctx,
		updatedTree.RegisteredBy,
	)

	if errors.Is(err, mongo.ErrNoDocuments) {
		registeredBy = nil
	} else if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "El árbol se actualizó, pero no pudo recuperarse el usuario",
			},
		)
		return
	}

	c.JSON(
		http.StatusOK,
		gin.H{
			"message": "Árbol actualizado correctamente",
			"tree": dto.NewTreeResponse(
				updatedTree,
				registeredBy,
			),
		},
	)
}

// UpdateStatus procesa:
//
// PATCH /api/v1/trees/:id/status
func (handler *TreeHandler) UpdateStatus(
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

	var request dto.UpdateTreeStatusRequest

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

	newStatus := models.TreeStatus(
		strings.ToUpper(
			strings.TrimSpace(
				string(request.Status),
			),
		),
	)

	if !isValidTreeStatus(newStatus) {
		c.JSON(
			http.StatusBadRequest,
			gin.H{
				"status":  "error",
				"message": "El estado debe ser ACTIVE, INACTIVE o ARCHIVED",
			},
		)
		return
	}

	if tree.Status == models.TreeStatusArchived &&
		newStatus != models.TreeStatusArchived {
		c.JSON(
			http.StatusConflict,
			gin.H{
				"status":  "error",
				"message": "Un árbol archivado no puede reactivarse",
			},
		)
		return
	}

	now := time.Now().UTC()

	if newStatus == models.TreeStatusActive {
		if err := validateCrewAllowsTreeChanges(
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
	}

	fields := bson.M{
		"status":    newStatus,
		"updatedAt": now,
	}

	if newStatus == models.TreeStatusArchived {
		fields["archivedAt"] = now
	}

	ctx, cancel := context.WithTimeout(
		c.Request.Context(),
		10*time.Second,
	)
	defer cancel()

	result, err := handler.trees.UpdateOne(
		ctx,
		bson.M{
			"_id": tree.ID,
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
				"message": "No fue posible actualizar el estado del árbol",
			},
		)
		return
	}

	if result.MatchedCount == 0 {
		c.JSON(
			http.StatusNotFound,
			gin.H{
				"status":  "error",
				"message": "El árbol no existe",
			},
		)
		return
	}

	var updatedTree models.Tree

	err = handler.trees.FindOne(
		ctx,
		bson.M{
			"_id": tree.ID,
		},
	).Decode(&updatedTree)
	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "El estado cambió, pero el árbol no pudo recuperarse",
			},
		)
		return
	}

	registeredBy, err := handler.findTreeRegisteredUser(
		ctx,
		updatedTree.RegisteredBy,
	)

	if errors.Is(err, mongo.ErrNoDocuments) {
		registeredBy = nil
	} else if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "El estado cambió, pero no pudo recuperarse el usuario",
			},
		)
		return
	}

	c.JSON(
		http.StatusOK,
		gin.H{
			"message": "Estado del árbol actualizado correctamente",
			"tree": dto.NewTreeResponse(
				updatedTree,
				registeredBy,
			),
		},
	)
}

func (handler *TreeHandler) findTreeRegisteredUser(
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

func (handler *TreeHandler) loadTreeRegisteredUsers(
	ctx context.Context,
	trees []models.Tree,
) (map[bson.ObjectID]models.User, error) {
	result := make(
		map[bson.ObjectID]models.User,
	)

	if len(trees) == 0 {
		return result, nil
	}

	userIDs := make(
		[]bson.ObjectID,
		0,
		len(trees),
	)

	seen := make(
		map[bson.ObjectID]struct{},
	)

	for _, tree := range trees {
		if _, exists := seen[tree.RegisteredBy]; exists {
			continue
		}

		seen[tree.RegisteredBy] = struct{}{}

		userIDs = append(
			userIDs,
			tree.RegisteredBy,
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

func normalizeTreeCode(
	code string,
) string {
	code = strings.TrimSpace(code)
	code = strings.ToUpper(code)
	code = strings.ReplaceAll(
		code,
		" ",
		"-",
	)

	return code
}

func validateTreeFields(
	code string,
	commonName string,
	scientificName string,
	latitude float64,
	longitude float64,
	locationDescription string,
) error {
	if !treeCodePattern.MatchString(code) {
		return fmt.Errorf(
			"el código solo puede contener letras mayúsculas, números y guiones, con una longitud de 3 a 40 caracteres",
		)
	}

	if len(strings.TrimSpace(commonName)) < 2 {
		return fmt.Errorf(
			"el nombre común debe tener al menos 2 caracteres",
		)
	}

	if len(commonName) > 120 {
		return fmt.Errorf(
			"el nombre común no puede superar 120 caracteres",
		)
	}

	if len(scientificName) > 160 {
		return fmt.Errorf(
			"el nombre científico no puede superar 160 caracteres",
		)
	}

	if latitude < -90 || latitude > 90 {
		return fmt.Errorf(
			"la latitud debe estar entre -90 y 90",
		)
	}

	if longitude < -180 || longitude > 180 {
		return fmt.Errorf(
			"la longitud debe estar entre -180 y 180",
		)
	}

	if len(locationDescription) > 500 {
		return fmt.Errorf(
			"la descripción de ubicación no puede superar 500 caracteres",
		)
	}

	return nil
}

func validateCrewAllowsTreeChanges(
	crew *models.Crew,
	currentUser *models.User,
	now time.Time,
) error {
	if crew.Status == models.CrewStatusFinished {
		return fmt.Errorf(
			"no se pueden modificar árboles de una cuadrilla finalizada",
		)
	}

	if crew.Status == models.CrewStatusCancelled {
		return fmt.Errorf(
			"no se pueden modificar árboles de una cuadrilla cancelada",
		)
	}

	if !crew.EndAt.After(now) {
		return fmt.Errorf(
			"la vigencia de la cuadrilla ya terminó",
		)
	}

	// El administrador y el encargado pueden preparar
	// árboles antes de que inicie formalmente la cuadrilla.
	if currentUser.Role == models.RoleSuperAdmin ||
		currentUser.Role == models.RoleManager {
		return nil
	}

	if currentUser.Role == models.RoleStudent {
		if now.Before(crew.StartAt) {
			return fmt.Errorf(
				"la cuadrilla todavía no inicia actividades",
			)
		}

		return nil
	}

	return fmt.Errorf(
		"tu rol no puede modificar árboles",
	)
}

func isValidTreeStatus(
	status models.TreeStatus,
) bool {
	switch status {
	case models.TreeStatusActive,
		models.TreeStatusInactive,
		models.TreeStatusArchived:
		return true

	default:
		return false
	}
}

func parseTreePositiveInteger(
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
