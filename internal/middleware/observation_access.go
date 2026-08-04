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

const CurrentObservationKey = "currentObservation"

// RequireObservationAccess valida el acceso de lectura
// a una observación.
func RequireObservationAccess(
	mongodb *database.MongoDB,
) gin.HandlerFunc {
	return requireObservationAccess(
		mongodb,
		false,
	)
}

// RequireObservationWriteAccess valida el acceso de edición.
//
// Un estudiante solamente puede editar observaciones
// registradas por él mismo.
func RequireObservationWriteAccess(
	mongodb *database.MongoDB,
) gin.HandlerFunc {
	return requireObservationAccess(
		mongodb,
		true,
	)
}

func requireObservationAccess(
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

		observationIDText := strings.TrimSpace(
			c.Param("id"),
		)

		observationID, err := bson.ObjectIDFromHex(
			observationIDText,
		)
		if err != nil {
			c.AbortWithStatusJSON(
				http.StatusBadRequest,
				gin.H{
					"status":  "error",
					"message": "El identificador de la observación no es válido",
				},
			)
			return
		}

		ctx, cancel := context.WithTimeout(
			c.Request.Context(),
			8*time.Second,
		)
		defer cancel()

		var observation models.Observation

		err = mongodb.Collection("observations").
			FindOne(
				ctx,
				bson.M{
					"_id": observationID,
				},
			).
			Decode(&observation)

		if errors.Is(err, mongo.ErrNoDocuments) {
			c.AbortWithStatusJSON(
				http.StatusNotFound,
				gin.H{
					"status":  "error",
					"message": "La observación no existe",
				},
			)
			return
		}

		if err != nil {
			c.AbortWithStatusJSON(
				http.StatusInternalServerError,
				gin.H{
					"status":  "error",
					"message": "No fue posible consultar la observación",
				},
			)
			return
		}

		var tree models.Tree

		err = mongodb.Collection("trees").
			FindOne(
				ctx,
				bson.M{
					"_id": observation.TreeID,
				},
			).
			Decode(&tree)

		if errors.Is(err, mongo.ErrNoDocuments) {
			c.AbortWithStatusJSON(
				http.StatusNotFound,
				gin.H{
					"status":  "error",
					"message": "El árbol asociado ya no existe",
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
			observation.CrewID,
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
					"message": "No fue posible validar el acceso",
				},
			)
			return
		}

		if !hasAccess {
			c.AbortWithStatusJSON(
				http.StatusForbidden,
				gin.H{
					"status":  "error",
					"message": "No tienes acceso a esta observación",
				},
			)
			return
		}

		if requireOwnerForStudent &&
			currentUser.Role == models.RoleStudent &&
			observation.ObserverID != currentUser.ID {
			c.AbortWithStatusJSON(
				http.StatusForbidden,
				gin.H{
					"status":  "error",
					"message": "Solo puedes modificar las observaciones que registraste",
				},
			)
			return
		}

		c.Set(
			CurrentObservationKey,
			&observation,
		)

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

// GetCurrentObservation recupera la observación validada.
func GetCurrentObservation(
	c *gin.Context,
) (*models.Observation, bool) {
	value, exists := c.Get(
		CurrentObservationKey,
	)
	if !exists {
		return nil, false
	}

	observation, ok := value.(*models.Observation)
	if !ok || observation == nil {
		return nil, false
	}

	return observation, true
}
