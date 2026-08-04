package handlers

import (
	"context"
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

// ManagerPortalHandler gestiona las consultas
// personalizadas del encargado.
type ManagerPortalHandler struct {
	crews        *mongo.Collection
	memberships  *mongo.Collection
	trees        *mongo.Collection
	observations *mongo.Collection
	images       *mongo.Collection
}

func NewManagerPortalHandler(
	mongodb *database.MongoDB,
) *ManagerPortalHandler {
	return &ManagerPortalHandler{
		crews: mongodb.Collection(
			"crews",
		),
		memberships: mongodb.Collection(
			"crew_memberships",
		),
		trees: mongodb.Collection(
			"trees",
		),
		observations: mongodb.Collection(
			"observations",
		),
		images: mongodb.Collection(
			"observation_images",
		),
	}
}

// Dashboard procesa:
//
// GET /api/v1/manager/dashboard
func (handler *ManagerPortalHandler) Dashboard(
	c *gin.Context,
) {
	currentUser, ok :=
		middleware.GetCurrentUser(c)

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
		25*time.Second,
	)
	defer cancel()

	managerFilter := bson.M{
		"managerId": currentUser.ID,
	}

	assignedCrews, err :=
		handler.crews.CountDocuments(
			ctx,
			managerFilter,
		)

	if err != nil {
		managerPortalInternalError(
			c,
			"No fue posible contar las cuadrillas",
		)
		return
	}

	pendingCrews, err :=
		handler.countManagerCrewsByStatus(
			ctx,
			currentUser.ID,
			models.CrewStatusPending,
		)

	if err != nil {
		managerPortalInternalError(
			c,
			"No fue posible contar las cuadrillas pendientes",
		)
		return
	}

	activeCrews, err :=
		handler.countManagerCrewsByStatus(
			ctx,
			currentUser.ID,
			models.CrewStatusActive,
		)

	if err != nil {
		managerPortalInternalError(
			c,
			"No fue posible contar las cuadrillas activas",
		)
		return
	}

	finishedCrews, err :=
		handler.countManagerCrewsByStatus(
			ctx,
			currentUser.ID,
			models.CrewStatusFinished,
		)

	if err != nil {
		managerPortalInternalError(
			c,
			"No fue posible contar las cuadrillas finalizadas",
		)
		return
	}

	cancelledCrews, err :=
		handler.countManagerCrewsByStatus(
			ctx,
			currentUser.ID,
			models.CrewStatusCancelled,
		)

	if err != nil {
		managerPortalInternalError(
			c,
			"No fue posible contar las cuadrillas canceladas",
		)
		return
	}

	currentCrews, err :=
		handler.findManagerCurrentCrews(
			ctx,
			currentUser.ID,
			0,
			5,
		)

	if err != nil {
		managerPortalInternalError(
			c,
			"No fue posible recuperar las cuadrillas vigentes",
		)
		return
	}

	crewResponses := make(
		[]dto.ManagerCurrentCrewResponse,
		0,
		len(currentCrews),
	)

	var currentStudents int64
	var currentTrees int64
	var currentObservations int64
	var currentImages int64

	for _, crew := range currentCrews {
		crewResponse, err :=
			handler.buildManagerCrewResponse(
				ctx,
				crew,
			)

		if err != nil {
			managerPortalInternalError(
				c,
				"No fue posible recuperar las estadísticas",
			)
			return
		}

		currentStudents +=
			crewResponse.Stats.Students

		currentTrees +=
			crewResponse.Stats.Trees

		currentObservations +=
			crewResponse.Stats.Observations

		currentImages +=
			crewResponse.Stats.Images

		crewResponses = append(
			crewResponses,
			crewResponse,
		)
	}

	c.JSON(
		http.StatusOK,
		dto.ManagerDashboardResponse{
			Manager: newPortalUserResponse(
				*currentUser,
			),
			Summary: dto.ManagerDashboardSummary{
				AssignedCrews:  assignedCrews,
				PendingCrews:   pendingCrews,
				ActiveCrews:    activeCrews,
				FinishedCrews:  finishedCrews,
				CancelledCrews: cancelledCrews,

				CurrentStudents:     currentStudents,
				CurrentTrees:        currentTrees,
				CurrentObservations: currentObservations,
				CurrentImages:       currentImages,
			},
			CurrentCrews: crewResponses,
		},
	)
}

// CurrentCrews procesa:
//
// GET /api/v1/manager/current-crews
//
// Parámetros:
//
// ?page=1
// ?limit=10
func (handler *ManagerPortalHandler) CurrentCrews(
	c *gin.Context,
) {
	currentUser, ok :=
		middleware.GetCurrentUser(c)

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

	page, valid :=
		parseManagerPortalPositiveInteger(
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

	limit, valid :=
		parseManagerPortalPositiveInteger(
			c.Query("limit"),
			10,
		)

	if !valid || limit > 50 {
		c.JSON(
			http.StatusBadRequest,
			gin.H{
				"status":  "error",
				"message": "limit debe estar entre 1 y 50",
			},
		)
		return
	}

	now := time.Now().UTC()

	filter := managerCurrentCrewsFilter(
		currentUser.ID,
		now,
	)

	ctx, cancel := context.WithTimeout(
		c.Request.Context(),
		25*time.Second,
	)
	defer cancel()

	total, err := handler.crews.CountDocuments(
		ctx,
		filter,
	)

	if err != nil {
		managerPortalInternalError(
			c,
			"No fue posible contar las cuadrillas vigentes",
		)
		return
	}

	skip := int64(
		(page - 1) * limit,
	)

	crews, err :=
		handler.findManagerCurrentCrews(
			ctx,
			currentUser.ID,
			skip,
			int64(limit),
		)

	if err != nil {
		managerPortalInternalError(
			c,
			"No fue posible consultar las cuadrillas vigentes",
		)
		return
	}

	responses := make(
		[]dto.ManagerCurrentCrewResponse,
		0,
		len(crews),
	)

	for _, crew := range crews {
		response, err :=
			handler.buildManagerCrewResponse(
				ctx,
				crew,
			)

		if err != nil {
			managerPortalInternalError(
				c,
				"No fue posible recuperar las estadísticas",
			)
			return
		}

		responses = append(
			responses,
			response,
		)
	}

	totalPages := int64(0)

	if total > 0 {
		totalPages = (total + int64(limit) - 1) / int64(limit)
	}

	c.JSON(
		http.StatusOK,
		dto.ManagerCurrentCrewsResponse{
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

func (
	handler *ManagerPortalHandler,
) countManagerCrewsByStatus(
	ctx context.Context,
	managerID bson.ObjectID,
	status models.CrewStatus,
) (int64, error) {
	return handler.crews.CountDocuments(
		ctx,
		bson.M{
			"managerId": managerID,
			"status":    status,
		},
	)
}

func (
	handler *ManagerPortalHandler,
) findManagerCurrentCrews(
	ctx context.Context,
	managerID bson.ObjectID,
	skip int64,
	limit int64,
) ([]models.Crew, error) {
	filter := managerCurrentCrewsFilter(
		managerID,
		time.Now().UTC(),
	)

	cursor, err := handler.crews.Find(
		ctx,
		filter,
		options.Find().
			SetSort(
				bson.D{
					{
						Key:   "startAt",
						Value: 1,
					},
				},
			).
			SetSkip(skip).
			SetLimit(limit),
	)

	if err != nil {
		return nil, err
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
		return nil, err
	}

	return crews, nil
}

func (
	handler *ManagerPortalHandler,
) buildManagerCrewResponse(
	ctx context.Context,
	crew models.Crew,
) (dto.ManagerCurrentCrewResponse, error) {
	now := time.Now().UTC()

	students, err :=
		handler.memberships.CountDocuments(
			ctx,
			bson.M{
				"crewId": crew.ID,
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

	if err != nil {
		return dto.ManagerCurrentCrewResponse{}, err
	}

	trees, err :=
		handler.trees.CountDocuments(
			ctx,
			bson.M{
				"crewId": crew.ID,
				"status": models.TreeStatusActive,
			},
		)

	if err != nil {
		return dto.ManagerCurrentCrewResponse{}, err
	}

	observations, err :=
		handler.observations.CountDocuments(
			ctx,
			bson.M{
				"crewId": crew.ID,
				"status": models.ObservationStatusActive,
			},
		)

	if err != nil {
		return dto.ManagerCurrentCrewResponse{}, err
	}

	images, err :=
		handler.images.CountDocuments(
			ctx,
			bson.M{
				"crewId": crew.ID,
			},
		)

	if err != nil {
		return dto.ManagerCurrentCrewResponse{}, err
	}

	return dto.ManagerCurrentCrewResponse{
		Crew: newPortalCrewResponse(
			crew,
		),
		Stats: dto.PortalCrewStats{
			Students:     students,
			Trees:        trees,
			Observations: observations,
			Images:       images,
		},
	}, nil
}

func managerCurrentCrewsFilter(
	managerID bson.ObjectID,
	now time.Time,
) bson.M {
	return bson.M{
		"managerId": managerID,
		"status": bson.M{
			"$in": bson.A{
				models.CrewStatusPending,
				models.CrewStatusActive,
			},
		},
		"endAt": bson.M{
			"$gt": now,
		},
	}
}

func parseManagerPortalPositiveInteger(
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

func managerPortalInternalError(
	c *gin.Context,
	message string,
) {
	c.JSON(
		http.StatusInternalServerError,
		gin.H{
			"status":  "error",
			"message": message,
		},
	)
}
