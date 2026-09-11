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

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// StudentPortalHandler gestiona las consultas
// personalizadas del alumno autenticado.
type StudentPortalHandler struct {
	users        *mongo.Collection
	crews        *mongo.Collection
	memberships  *mongo.Collection
	trees        *mongo.Collection
	observations *mongo.Collection
	images       *mongo.Collection
}

func NewStudentPortalHandler(
	mongodb *database.MongoDB,
) *StudentPortalHandler {
	return &StudentPortalHandler{
		users: mongodb.Collection(
			"users",
		),
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

// Profile procesa:
//
// GET /api/v1/student/profile
func (handler *StudentPortalHandler) Profile(
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
		12*time.Second,
	)
	defer cancel()

	membership, crew, err :=
		handler.findStudentCurrentContext(
			ctx,
			currentUser.ID,
		)

	response := dto.StudentProfileResponse{
		Student: newPortalUserResponse(
			*currentUser,
		),
	}

	if errors.Is(err, mongo.ErrNoDocuments) {
		c.JSON(
			http.StatusOK,
			response,
		)
		return
	}

	if err != nil {
		studentPortalInternalError(
			c,
			"No fue posible consultar la asignación del alumno",
		)
		return
	}

	membershipResponse :=
		newPortalMembershipResponse(
			*membership,
		)

	crewResponse :=
		newPortalCrewResponse(
			*crew,
		)

	response.CurrentMembership =
		&membershipResponse

	response.CurrentCrew =
		&crewResponse

	c.JSON(
		http.StatusOK,
		response,
	)
}

// CurrentCrew procesa:
//
// GET /api/v1/student/current-crew
func (handler *StudentPortalHandler) CurrentCrew(
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
		20*time.Second,
	)
	defer cancel()

	membership, crew, err :=
		handler.findStudentCurrentContext(
			ctx,
			currentUser.ID,
		)

	if errors.Is(err, mongo.ErrNoDocuments) {
		c.JSON(
			http.StatusNotFound,
			gin.H{
				"status":  "error",
				"message": "El alumno no tiene una cuadrilla vigente",
			},
		)
		return
	}

	if err != nil {
		studentPortalInternalError(
			c,
			"No fue posible consultar la cuadrilla vigente",
		)
		return
	}

	var managerResponse *dto.PortalUserResponse

	var manager models.User

	err = handler.users.FindOne(
		ctx,
		bson.M{
			"_id": crew.ManagerID,
		},
	).Decode(&manager)

	if err == nil {
		response :=
			newPortalUserResponse(
				manager,
			)

		managerResponse = &response
	} else if !errors.Is(
		err,
		mongo.ErrNoDocuments,
	) {
		studentPortalInternalError(
			c,
			"No fue posible consultar al encargado",
		)
		return
	}

	stats, err := handler.buildStudentCrewStats(
		ctx,
		crew.ID,
	)

	if err != nil {
		studentPortalInternalError(
			c,
			"No fue posible recuperar las estadísticas de la cuadrilla",
		)
		return
	}

	c.JSON(
		http.StatusOK,
		dto.StudentCurrentCrewResponse{
			Crew: newPortalCrewResponse(
				*crew,
			),
			Membership: newPortalMembershipResponse(
				*membership,
			),
			Manager: managerResponse,
			Stats:   stats,
		},
	)
}

// Trees procesa:
//
// GET /api/v1/student/trees
//
// Parámetros:
//
// ?page=1
// ?limit=10
// ?search=HM-001
func (handler *StudentPortalHandler) Trees(
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

	page, limit, valid :=
		parseStudentPortalPagination(c)

	if !valid {
		return
	}

	ctx, cancel := context.WithTimeout(
		c.Request.Context(),
		20*time.Second,
	)
	defer cancel()

	_, crew, err :=
		handler.findStudentCurrentContext(
			ctx,
			currentUser.ID,
		)

	if errors.Is(err, mongo.ErrNoDocuments) {
		c.JSON(
			http.StatusNotFound,
			gin.H{
				"status":  "error",
				"message": "El alumno no tiene una cuadrilla vigente",
			},
		)
		return
	}

	if err != nil {
		studentPortalInternalError(
			c,
			"No fue posible consultar la cuadrilla vigente",
		)
		return
	}

	filter := bson.M{
		"crewId": crew.ID,
		"status": models.TreeStatusActive,
	}

	search := strings.TrimSpace(
		c.Query("search"),
	)

	if search != "" {
		safeSearch := regexp.QuoteMeta(
			search,
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
		}
	}

	total, err := handler.trees.CountDocuments(
		ctx,
		filter,
	)

	if err != nil {
		studentPortalInternalError(
			c,
			"No fue posible contar los árboles",
		)
		return
	}

	skip := int64(
		(page - 1) * limit,
	)

	cursor, err := handler.trees.Find(
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
		studentPortalInternalError(
			c,
			"No fue posible consultar los árboles",
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
		studentPortalInternalError(
			c,
			"No fue posible procesar los árboles",
		)
		return
	}

	responses := make(
		[]dto.StudentTreeResponse,
		0,
		len(trees),
	)

	for _, tree := range trees {
		responses = append(
			responses,
			dto.StudentTreeResponse{
				ID: tree.ID.Hex(),

				CrewID: tree.CrewID.Hex(),

				Code:           tree.Code,
				CommonName:     tree.CommonName,
				ScientificName: tree.ScientificName,

				Latitude:  tree.Latitude,
				Longitude: tree.Longitude,

				LocationDescription: tree.LocationDescription,

				Status: string(tree.Status),

				RegisteredBy: tree.RegisteredBy.Hex(),

				RegisteredByMe: tree.RegisteredBy ==
					currentUser.ID,

				CreatedAt: tree.CreatedAt,
				UpdatedAt: tree.UpdatedAt,
			},
		)
	}

	totalPages := int64(0)

	if total > 0 {
		totalPages = (total + int64(limit) - 1) / int64(limit)
	}

	c.JSON(
		http.StatusOK,
		dto.StudentTreeListResponse{
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

// Observations procesa:
//
// GET /api/v1/student/observations
//
// Devuelve solamente las observaciones creadas
// por el alumno autenticado.
//
// Parámetros:
//
// ?page=1
// ?limit=10
// ?treeId=OBJECT_ID
func (handler *StudentPortalHandler) Observations(
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

	page, limit, valid :=
		parseStudentPortalPagination(c)

	if !valid {
		return
	}

	ctx, cancel := context.WithTimeout(
		c.Request.Context(),
		20*time.Second,
	)
	defer cancel()

	_, crew, err :=
		handler.findStudentCurrentContext(
			ctx,
			currentUser.ID,
		)

	if errors.Is(err, mongo.ErrNoDocuments) {
		c.JSON(
			http.StatusNotFound,
			gin.H{
				"status":  "error",
				"message": "El alumno no tiene una cuadrilla vigente",
			},
		)
		return
	}

	if err != nil {
		studentPortalInternalError(
			c,
			"No fue posible consultar la cuadrilla vigente",
		)
		return
	}

	filter := bson.M{
		"crewId":     crew.ID,
		"observerId": currentUser.ID,
		"status":     models.ObservationStatusActive,
	}

	treeIDText := strings.TrimSpace(
		c.Query("treeId"),
	)

	if treeIDText != "" {
		treeID, err :=
			bson.ObjectIDFromHex(
				treeIDText,
			)

		if err != nil {
			c.JSON(
				http.StatusBadRequest,
				gin.H{
					"status":  "error",
					"message": "treeId no es válido",
				},
			)
			return
		}

		filter["treeId"] = treeID
	}

	total, err :=
		handler.observations.CountDocuments(
			ctx,
			filter,
		)

	if err != nil {
		studentPortalInternalError(
			c,
			"No fue posible contar las observaciones",
		)
		return
	}

	skip := int64(
		(page - 1) * limit,
	)

	cursor, err :=
		handler.observations.Find(
			ctx,
			filter,
			options.Find().
				SetSort(
					bson.D{
						{
							Key:   "observationDate",
							Value: -1,
						},
					},
				).
				SetSkip(skip).
				SetLimit(int64(limit)),
		)

	if err != nil {
		studentPortalInternalError(
			c,
			"No fue posible consultar las observaciones",
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
		studentPortalInternalError(
			c,
			"No fue posible procesar las observaciones",
		)
		return
	}

	treesMap, err :=
		handler.loadStudentObservationTrees(
			ctx,
			observations,
		)

	if err != nil {
		studentPortalInternalError(
			c,
			"No fue posible recuperar los árboles",
		)
		return
	}

	responses := make(
		[]dto.StudentObservationResponse,
		0,
		len(observations),
	)

	for _, observation := range observations {
		var treeResponse *dto.PortalTreeSummaryResponse

		tree, exists :=
			treesMap[observation.TreeID]

		if exists {
			treeResponse =
				&dto.PortalTreeSummaryResponse{
					ID: tree.ID.Hex(),

					Code: tree.Code,

					CommonName: tree.CommonName,

					ScientificName: tree.ScientificName,
				}
		}

		responses = append(
			responses,
			dto.StudentObservationResponse{
				ID: observation.ID.Hex(),

				TreeID: observation.TreeID.Hex(),
				CrewID: observation.CrewID.Hex(),

				Tree: treeResponse,

				Method: string(
					observation.Method,
				),

				LowerThirdScore: observation.LowerThirdScore,

				MiddleThirdScore: observation.MiddleThirdScore,

				UpperThirdScore: observation.UpperThirdScore,

				TotalScore: observation.TotalScore,

				Notes: observation.Notes,

				ObservationDate: observation.ObservationDate,

				Latitude: observation.Latitude,

				Longitude: observation.Longitude,

				Status: string(
					observation.Status,
				),

				CreatedAt: observation.CreatedAt,

				UpdatedAt: observation.UpdatedAt,
			},
		)
	}

	totalPages := int64(0)

	if total > 0 {
		totalPages = (total + int64(limit) - 1) / int64(limit)
	}

	c.JSON(
		http.StatusOK,
		dto.StudentObservationListResponse{
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

func (
	handler *StudentPortalHandler,
) findStudentCurrentContext(
	ctx context.Context,
	studentID bson.ObjectID,
) (
	*models.CrewMembership,
	*models.Crew,
	error,
) {
	now := time.Now().UTC()

	var membership models.CrewMembership

	err := handler.memberships.FindOne(
		ctx,
		bson.M{
			"studentId": studentID,
			"status":    models.MembershipStatusActive,
			"validFrom": bson.M{
				"$lte": now,
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
		return nil, nil, err
	}

	var crew models.Crew

	err = handler.crews.FindOne(
		ctx,
		bson.M{
			"_id": membership.CrewID,
			"status": bson.M{
				"$nin": bson.A{
					models.CrewStatusFinished,
					models.CrewStatusCancelled,
				},
			},
			"startAt": bson.M{
				"$lte": now,
			},
			"endAt": bson.M{
				"$gt": now,
			},
		},
	).Decode(&crew)

	if err != nil {
		return nil, nil, err
	}

	return &membership, &crew, nil
}

func (
	handler *StudentPortalHandler,
) buildStudentCrewStats(
	ctx context.Context,
	crewID bson.ObjectID,
) (dto.PortalCrewStats, error) {
	now := time.Now().UTC()

	students, err :=
		handler.memberships.CountDocuments(
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

	if err != nil {
		return dto.PortalCrewStats{}, err
	}

	trees, err :=
		handler.trees.CountDocuments(
			ctx,
			bson.M{
				"crewId": crewID,
				"status": models.TreeStatusActive,
			},
		)

	if err != nil {
		return dto.PortalCrewStats{}, err
	}

	observations, err :=
		handler.observations.CountDocuments(
			ctx,
			bson.M{
				"crewId": crewID,
				"status": models.ObservationStatusActive,
			},
		)

	if err != nil {
		return dto.PortalCrewStats{}, err
	}

	images, err :=
		handler.images.CountDocuments(
			ctx,
			bson.M{
				"crewId": crewID,
			},
		)

	if err != nil {
		return dto.PortalCrewStats{}, err
	}

	return dto.PortalCrewStats{
		Students:     students,
		Trees:        trees,
		Observations: observations,
		Images:       images,
	}, nil
}

func (
	handler *StudentPortalHandler,
) loadStudentObservationTrees(
	ctx context.Context,
	observations []models.Observation,
) (
	map[bson.ObjectID]models.Tree,
	error,
) {
	result := make(
		map[bson.ObjectID]models.Tree,
	)

	if len(observations) == 0 {
		return result, nil
	}

	treeIDs := make(
		[]bson.ObjectID,
		0,
		len(observations),
	)

	seen := make(
		map[bson.ObjectID]struct{},
	)

	for _, observation := range observations {
		if _, exists :=
			seen[observation.TreeID]; exists {
			continue
		}

		seen[observation.TreeID] =
			struct{}{}

		treeIDs = append(
			treeIDs,
			observation.TreeID,
		)
	}

	cursor, err := handler.trees.Find(
		ctx,
		bson.M{
			"_id": bson.M{
				"$in": treeIDs,
			},
		},
	)

	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var trees []models.Tree

	if err := cursor.All(
		ctx,
		&trees,
	); err != nil {
		return nil, err
	}

	for _, tree := range trees {
		result[tree.ID] = tree
	}

	return result, nil
}

func parseStudentPortalPagination(
	c *gin.Context,
) (int, int, bool) {
	page, valid :=
		parseStudentPortalPositiveInteger(
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

		return 0, 0, false
	}

	limit, valid :=
		parseStudentPortalPositiveInteger(
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

		return 0, 0, false
	}

	return page, limit, true
}

func parseStudentPortalPositiveInteger(
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

func studentPortalInternalError(
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

// Funciones compartidas por los paneles.

func newPortalUserResponse(
	user models.User,
) dto.PortalUserResponse {
	return dto.PortalUserResponse{
		ID:         user.ID.Hex(),
		Name:       user.Name,
		Email:      user.Email,
		Enrollment: user.Enrollment,
		Role:       string(user.Role),
		Status:     string(user.Status),
	}
}

func newPortalCrewResponse(
	crew models.Crew,
) dto.PortalCrewResponse {
	return dto.PortalCrewResponse{
		ID: crew.ID.Hex(),

		Name:        crew.Name,
		Zone:        crew.Zone,
		Institution: crew.Institution,

		ManagerID: crew.ManagerID.Hex(),

		Status: string(crew.Status),

		StartAt: crew.StartAt,
		EndAt:   crew.EndAt,

		StudentLimit: crew.StudentLimit,
	}
}

func newPortalMembershipResponse(
	membership models.CrewMembership,
) dto.PortalMembershipResponse {
	return dto.PortalMembershipResponse{
		ID: membership.ID.Hex(),

		CrewID:    membership.CrewID.Hex(),
		StudentID: membership.StudentID.Hex(),

		Status: string(membership.Status),

		ValidFrom:  membership.ValidFrom,
		ValidUntil: membership.ValidUntil,

		ActivatedAt: membership.ActivatedAt,

		CreatedAt: membership.CreatedAt,
		UpdatedAt: membership.UpdatedAt,
	}
}
