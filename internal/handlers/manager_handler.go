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
	"heno-motita-api/internal/models"
	"heno-motita-api/internal/security"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// ManagerHandler contiene la colección de usuarios.
type ManagerHandler struct {
	users *mongo.Collection
}

// NewManagerHandler crea el controlador de encargados.
func NewManagerHandler(
	mongodb *database.MongoDB,
) *ManagerHandler {
	return &ManagerHandler{
		users: mongodb.Collection("users"),
	}
}

// Create procesa:
//
// POST /api/v1/managers
func (handler *ManagerHandler) Create(
	c *gin.Context,
) {
	var request dto.CreateManagerRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(
			http.StatusBadRequest,
			gin.H{
				"status":  "error",
				"message": "Los datos del encargado no son válidos",
				"details": err.Error(),
			},
		)
		return
	}

	name := strings.TrimSpace(request.Name)
	email := strings.ToLower(
		strings.TrimSpace(request.Email),
	)
	phone := strings.TrimSpace(request.Phone)
	institution := strings.TrimSpace(request.Institution)

	if name == "" {
		c.JSON(
			http.StatusBadRequest,
			gin.H{
				"status":  "error",
				"message": "El nombre es obligatorio",
			},
		)
		return
	}

	if institution == "" {
		c.JSON(
			http.StatusBadRequest,
			gin.H{
				"status":  "error",
				"message": "La institución es obligatoria",
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

	now := time.Now().UTC()

	manager := models.User{
		ID:           bson.NewObjectID(),
		Name:         name,
		Email:        email,
		Phone:        phone,
		Institution:  institution,
		PasswordHash: passwordHash,
		Role:         models.RoleManager,
		Status:       models.StatusActive,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	ctx, cancel := context.WithTimeout(
		c.Request.Context(),
		8*time.Second,
	)
	defer cancel()

	_, err = handler.users.InsertOne(
		ctx,
		manager,
	)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			c.JSON(
				http.StatusConflict,
				gin.H{
					"status":  "error",
					"message": "Ya existe un usuario con ese correo",
				},
			)
			return
		}

		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "No fue posible registrar al encargado",
			},
		)
		return
	}

	c.JSON(
		http.StatusCreated,
		gin.H{
			"message": "Encargado registrado correctamente",
			"manager": dto.NewManagerResponse(manager),
		},
	)
}

// List procesa:
//
// GET /api/v1/managers
//
// Parámetros opcionales:
// ?page=1
// ?limit=10
// ?search=rodrigo
// ?status=ACTIVE
func (handler *ManagerHandler) List(
	c *gin.Context,
) {
	page, err := parsePositiveInteger(
		c.Query("page"),
		1,
	)
	if err != nil {
		c.JSON(
			http.StatusBadRequest,
			gin.H{
				"status":  "error",
				"message": "El parámetro page debe ser un entero mayor que cero",
			},
		)
		return
	}

	limit, err := parsePositiveInteger(
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

	filter := bson.M{
		"role": models.RoleManager,
	}

	statusText := strings.ToUpper(
		strings.TrimSpace(
			c.Query("status"),
		),
	)

	if statusText != "" {
		status := models.UserStatus(statusText)

		if !isValidManagerStatus(status) {
			c.JSON(
				http.StatusBadRequest,
				gin.H{
					"status":  "error",
					"message": "El estado debe ser ACTIVE, INACTIVE o BLOCKED",
				},
			)
			return
		}

		filter["status"] = status
	}

	search := strings.TrimSpace(
		c.Query("search"),
	)

	if search != "" {
		// QuoteMeta evita que el texto ingresado sea interpretado
		// como una expresión regular especial.
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
				"phone": bson.M{
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

	total, err := handler.users.CountDocuments(
		ctx,
		filter,
	)
	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "No fue posible contar los encargados",
			},
		)
		return
	}

	skip := int64((page - 1) * limit)

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

	cursor, err := handler.users.Find(
		ctx,
		filter,
		findOptions,
	)
	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "No fue posible consultar los encargados",
			},
		)
		return
	}
	defer cursor.Close(ctx)

	var managers []models.User

	if err := cursor.All(ctx, &managers); err != nil {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "No fue posible procesar los encargados",
			},
		)
		return
	}

	responses := make(
		[]dto.ManagerResponse,
		0,
		len(managers),
	)

	for _, manager := range managers {
		responses = append(
			responses,
			dto.NewManagerResponse(manager),
		)
	}

	totalPages := int64(0)

	if total > 0 {
		totalPages = (total + int64(limit) - 1) / int64(limit)
	}

	c.JSON(
		http.StatusOK,
		dto.ManagerListResponse{
			Managers: responses,
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
// GET /api/v1/managers/:id
func (handler *ManagerHandler) GetByID(
	c *gin.Context,
) {
	managerID, ok := managerIDFromParam(c)
	if !ok {
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
			"_id":  managerID,
			"role": models.RoleManager,
		},
	).Decode(&manager)

	if errors.Is(err, mongo.ErrNoDocuments) {
		c.JSON(
			http.StatusNotFound,
			gin.H{
				"status":  "error",
				"message": "El encargado no existe",
			},
		)
		return
	}

	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "No fue posible consultar al encargado",
			},
		)
		return
	}

	c.JSON(
		http.StatusOK,
		gin.H{
			"manager": dto.NewManagerResponse(manager),
		},
	)
}

// Update procesa:
//
// PUT /api/v1/managers/:id
func (handler *ManagerHandler) Update(
	c *gin.Context,
) {
	managerID, ok := managerIDFromParam(c)
	if !ok {
		return
	}

	var request dto.UpdateManagerRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(
			http.StatusBadRequest,
			gin.H{
				"status":  "error",
				"message": "Los datos del encargado no son válidos",
				"details": err.Error(),
			},
		)
		return
	}

	name := strings.TrimSpace(request.Name)
	email := strings.ToLower(
		strings.TrimSpace(request.Email),
	)
	phone := strings.TrimSpace(request.Phone)
	institution := strings.TrimSpace(request.Institution)

	if name == "" || institution == "" {
		c.JSON(
			http.StatusBadRequest,
			gin.H{
				"status":  "error",
				"message": "El nombre y la institución son obligatorios",
			},
		)
		return
	}

	fields := bson.M{
		"name":        name,
		"email":       email,
		"phone":       phone,
		"institution": institution,
		"updatedAt":   time.Now().UTC(),
	}

	// Solo modifica la contraseña cuando se envía una nueva.
	if request.Password != "" {
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

		fields["passwordHash"] = passwordHash
	}

	ctx, cancel := context.WithTimeout(
		c.Request.Context(),
		8*time.Second,
	)
	defer cancel()

	result, err := handler.users.UpdateOne(
		ctx,
		bson.M{
			"_id":  managerID,
			"role": models.RoleManager,
		},
		bson.M{
			"$set": fields,
		},
	)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			c.JSON(
				http.StatusConflict,
				gin.H{
					"status":  "error",
					"message": "Ya existe un usuario con ese correo",
				},
			)
			return
		}

		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "No fue posible actualizar al encargado",
			},
		)
		return
	}

	if result.MatchedCount == 0 {
		c.JSON(
			http.StatusNotFound,
			gin.H{
				"status":  "error",
				"message": "El encargado no existe",
			},
		)
		return
	}

	var updatedManager models.User

	err = handler.users.FindOne(
		ctx,
		bson.M{
			"_id":  managerID,
			"role": models.RoleManager,
		},
	).Decode(&updatedManager)
	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "El encargado se actualizó, pero no pudo recuperarse",
			},
		)
		return
	}

	c.JSON(
		http.StatusOK,
		gin.H{
			"message": "Encargado actualizado correctamente",
			"manager": dto.NewManagerResponse(updatedManager),
		},
	)
}

// UpdateStatus procesa:
//
// PATCH /api/v1/managers/:id/status
func (handler *ManagerHandler) UpdateStatus(
	c *gin.Context,
) {
	managerID, ok := managerIDFromParam(c)
	if !ok {
		return
	}

	var request dto.UpdateManagerStatusRequest

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

	if !isValidManagerStatus(status) {
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
		8*time.Second,
	)
	defer cancel()

	result, err := handler.users.UpdateOne(
		ctx,
		bson.M{
			"_id":  managerID,
			"role": models.RoleManager,
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
				"message": "El encargado no existe",
			},
		)
		return
	}

	var updatedManager models.User

	err = handler.users.FindOne(
		ctx,
		bson.M{
			"_id":  managerID,
			"role": models.RoleManager,
		},
	).Decode(&updatedManager)
	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "El estado cambió, pero no pudo recuperarse el encargado",
			},
		)
		return
	}

	c.JSON(
		http.StatusOK,
		gin.H{
			"message": "Estado actualizado correctamente",
			"manager": dto.NewManagerResponse(updatedManager),
		},
	)
}

func managerIDFromParam(
	c *gin.Context,
) (bson.ObjectID, bool) {
	idText := strings.TrimSpace(
		c.Param("id"),
	)

	managerID, err := bson.ObjectIDFromHex(idText)
	if err != nil {
		c.JSON(
			http.StatusBadRequest,
			gin.H{
				"status":  "error",
				"message": "El identificador del encargado no es válido",
			},
		)

		return bson.ObjectID{}, false
	}

	return managerID, true
}

func isValidManagerStatus(
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

func parsePositiveInteger(
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
