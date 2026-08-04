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

const CurrentCrewKey = "currentCrew"

// RequireCrewAccess comprueba que el usuario pueda consultar
// la cuadrilla indicada en el parámetro :id.
//
// SUPER_ADMIN:
// puede consultar cualquier cuadrilla.
//
// CREW_MANAGER:
// solamente puede consultar una cuadrilla cuyo managerId
// coincida con su identificador de usuario.
func RequireCrewAccess(
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
			5*time.Second,
		)
		defer cancel()

		var crew models.Crew

		err = mongodb.Collection("crews").
			FindOne(
				ctx,
				bson.M{
					"_id": crewID,
				},
			).
			Decode(&crew)

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
					"message": "No fue posible validar la cuadrilla",
				},
			)
			return
		}

		switch currentUser.Role {
		case models.RoleSuperAdmin:
			// El superadministrador tiene acceso completo.

		case models.RoleManager:
			if crew.ManagerID != currentUser.ID {
				c.AbortWithStatusJSON(
					http.StatusForbidden,
					gin.H{
						"status":  "error",
						"message": "No tienes acceso a esta cuadrilla",
					},
				)
				return
			}

		default:
			c.AbortWithStatusJSON(
				http.StatusForbidden,
				gin.H{
					"status":  "error",
					"message": "Tu rol no tiene acceso a esta cuadrilla",
				},
			)
			return
		}

		c.Set(
			CurrentCrewKey,
			&crew,
		)

		c.Next()
	}
}

// GetCurrentCrew recupera la cuadrilla validada
// por RequireCrewAccess.
func GetCurrentCrew(
	c *gin.Context,
) (*models.Crew, bool) {
	value, exists := c.Get(
		CurrentCrewKey,
	)
	if !exists {
		return nil, false
	}

	crew, ok := value.(*models.Crew)
	if !ok || crew == nil {
		return nil, false
	}

	return crew, true
}
