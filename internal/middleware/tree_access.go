package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"heno-motita-api/internal/database"
	"heno-motita-api/internal/models"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

const CurrentTreeKey = "currentTree"

// RequireCrewTreeAccess valida el acceso a una cuadrilla
// utilizando el parámetro :crewId.
//
// SUPER_ADMIN:
// puede acceder a cualquier cuadrilla.
//
// CREW_MANAGER:
// únicamente puede acceder a una cuadrilla asignada.
//
// STUDENT:
// debe tener una membresía activa y vigente.
func RequireCrewTreeAccess(
	mongodb *database.MongoDB,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		currentUser, ok := GetCurrentUser(c)
		if !ok {
			c.AbortWithStatusJSON(
				http.StatusUnauthorized,
				gin.H{
					"status":  "error",
					"message": "No se encontró una sesión válida",
				},
			)
			return
		}

		crewIDText := strings.TrimSpace(
			c.Param("id"),
		)

		crewID, err := bson.ObjectIDFromHex(
			crewIDText,
		)
		if err != nil {
			c.AbortWithStatusJSON(
				http.StatusBadRequest,
				gin.H{
					"status":  "error",
					"message": "El identificador de la cuadrilla no es válido",
				},
			)
			return
		}

		ctx, cancel := context.WithTimeout(
			c.Request.Context(),
			8*time.Second,
		)
		defer cancel()

		crew, err := findCrewForTreeAccess(
			ctx,
			mongodb,
			crewID,
		)
		if errors.Is(err, mongo.ErrNoDocuments) {
			c.AbortWithStatusJSON(
				http.StatusNotFound,
				gin.H{
					"status":  "error",
					"message": "La cuadrilla no existe",
				},
			)
			return
		}

		if err != nil {
			c.AbortWithStatusJSON(
				http.StatusInternalServerError,
				gin.H{
					"status":  "error",
					"message": "No fue posible consultar la cuadrilla",
				},
			)
			return
		}

		hasAccess, err := userHasCrewTreeAccess(
			ctx,
			mongodb,
			currentUser,
			crew,
		)
		if err != nil {
			c.AbortWithStatusJSON(
				http.StatusInternalServerError,
				gin.H{
					"status":  "error",
					"message": "No fue posible validar el acceso a la cuadrilla",
				},
			)
			return
		}

		if !hasAccess {
			c.AbortWithStatusJSON(
				http.StatusForbidden,
				gin.H{
					"status":  "error",
					"message": "No tienes acceso a los árboles de esta cuadrilla",
				},
			)
			return
		}

		// CurrentCrewKey fue declarado en crew_access.go.
		c.Set(
			CurrentCrewKey,
			crew,
		)

		c.Next()
	}
}

// RequireTreeAccess valida el acceso de lectura
// a un árbol.
func RequireTreeAccess(
	mongodb *database.MongoDB,
) gin.HandlerFunc {
	return requireTreeAccess(
		mongodb,
		false,
	)
}

// RequireTreeWriteAccess valida el acceso de edición.
//
// Los estudiantes solamente pueden editar árboles
// que ellos mismos hayan registrado.
func RequireTreeWriteAccess(
	mongodb *database.MongoDB,
) gin.HandlerFunc {
	return requireTreeAccess(
		mongodb,
		true,
	)
}

func requireTreeAccess(
	mongodb *database.MongoDB,
	requireOwnerForStudent bool,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		currentUser, ok := GetCurrentUser(c)
		if !ok {
			c.AbortWithStatusJSON(
				http.StatusUnauthorized,
				gin.H{
					"status":  "error",
					"message": "No se encontró una sesión válida",
				},
			)
			return
		}

		treeIDText := strings.TrimSpace(
			c.Param("id"),
		)

		treeID, err := bson.ObjectIDFromHex(
			treeIDText,
		)
		if err != nil {
			c.AbortWithStatusJSON(
				http.StatusBadRequest,
				gin.H{
					"status":  "error",
					"message": "El identificador del árbol no es válido",
				},
			)
			return
		}

		ctx, cancel := context.WithTimeout(
			c.Request.Context(),
			8*time.Second,
		)
		defer cancel()

		var tree models.Tree

		err = mongodb.Collection("trees").
			FindOne(
				ctx,
				bson.M{
					"_id": treeID,
				},
			).
			Decode(&tree)

		if errors.Is(err, mongo.ErrNoDocuments) {
			c.AbortWithStatusJSON(
				http.StatusNotFound,
				gin.H{
					"status":  "error",
					"message": "El árbol no existe",
				},
			)
			return
		}

		if err != nil {
			c.AbortWithStatusJSON(
				http.StatusInternalServerError,
				gin.H{
					"status":  "error",
					"message": "No fue posible consultar el árbol",
				},
			)
			return
		}

		crew, err := findCrewForTreeAccess(
			ctx,
			mongodb,
			tree.CrewID,
		)
		if errors.Is(err, mongo.ErrNoDocuments) {
			c.AbortWithStatusJSON(
				http.StatusNotFound,
				gin.H{
					"status":  "error",
					"message": "La cuadrilla asociada ya no existe",
				},
			)
			return
		}

		if err != nil {
			c.AbortWithStatusJSON(
				http.StatusInternalServerError,
				gin.H{
					"status":  "error",
					"message": "No fue posible consultar la cuadrilla del árbol",
				},
			)
			return
		}

		hasAccess, err := userHasCrewTreeAccess(
			ctx,
			mongodb,
			currentUser,
			crew,
		)
		if err != nil {
			c.AbortWithStatusJSON(
				http.StatusInternalServerError,
				gin.H{
					"status":  "error",
					"message": "No fue posible validar el acceso al árbol",
				},
			)
			return
		}

		if !hasAccess {
			c.AbortWithStatusJSON(
				http.StatusForbidden,
				gin.H{
					"status":  "error",
					"message": "No tienes acceso a este árbol",
				},
			)
			return
		}

		if requireOwnerForStudent &&
			currentUser.Role == models.RoleStudent &&
			tree.RegisteredBy != currentUser.ID {
			c.AbortWithStatusJSON(
				http.StatusForbidden,
				gin.H{
					"status":  "error",
					"message": "Solo puedes modificar los árboles que registraste",
				},
			)
			return
		}

		c.Set(
			CurrentTreeKey,
			&tree,
		)

		c.Set(
			CurrentCrewKey,
			crew,
		)

		c.Next()
	}
}

// GetCurrentTree recupera el árbol que fue validado
// por RequireTreeAccess o RequireTreeWriteAccess.
func GetCurrentTree(
	c *gin.Context,
) (*models.Tree, bool) {
	value, exists := c.Get(
		CurrentTreeKey,
	)
	if !exists {
		return nil, false
	}

	tree, ok := value.(*models.Tree)
	if !ok || tree == nil {
		return nil, false
	}

	return tree, true
}

func findCrewForTreeAccess(
	ctx context.Context,
	mongodb *database.MongoDB,
	crewID bson.ObjectID,
) (*models.Crew, error) {
	var crew models.Crew

	err := mongodb.Collection("crews").
		FindOne(
			ctx,
			bson.M{
				"_id": crewID,
			},
		).
		Decode(&crew)

	if err != nil {
		return nil, err
	}

	return &crew, nil
}

func userHasCrewTreeAccess(
	ctx context.Context,
	mongodb *database.MongoDB,
	currentUser *models.User,
	crew *models.Crew,
) (bool, error) {
	switch currentUser.Role {
	case models.RoleSuperAdmin:
		return true, nil

	case models.RoleManager:
		return crew.ManagerID == currentUser.ID, nil

	case models.RoleStudent:
		now := time.Now().UTC()

		if crew.Status == models.CrewStatusFinished ||
			crew.Status == models.CrewStatusCancelled {
			return false, nil
		}

		if now.Before(crew.StartAt) ||
			!now.Before(crew.EndAt) {
			return false, nil
		}

		var membership models.CrewMembership

		err := mongodb.Collection("crew_memberships").
			FindOne(
				ctx,
				bson.M{
					"crewId":    crew.ID,
					"studentId": currentUser.ID,
					"status":    models.MembershipStatusActive,
					"validFrom": bson.M{
						"$lte": now,
					},
					"validUntil": bson.M{
						"$gt": now,
					},
				},
			).
			Decode(&membership)

		if errors.Is(err, mongo.ErrNoDocuments) {
			return false, nil
		}

		if err != nil {
			return false, err
		}

		return true, nil

	default:
		return false, nil
	}
}
