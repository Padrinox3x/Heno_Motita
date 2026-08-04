package routes

import (
	"heno-motita-api/internal/database"
	"heno-motita-api/internal/handlers"
	"heno-motita-api/internal/middleware"
	"heno-motita-api/internal/models"
	"heno-motita-api/internal/security"

	"github.com/gin-gonic/gin"
)

// RegisterCrewRoutes registra las rutas del módulo
// de cuadrillas.
func RegisterCrewRoutes(
	router *gin.Engine,
	crewHandler *handlers.CrewHandler,
	jwtManager *security.JWTManager,
	mongodb *database.MongoDB,
) {
	crews := router.Group(
		"/api/v1/crews",
	)

	// Todas las rutas requieren JWT.
	crews.Use(
		middleware.RequireAuth(
			jwtManager,
			mongodb,
		),
	)

	{
		// Solamente SUPER_ADMIN puede crear cuadrillas.
		crews.POST(
			"",
			middleware.RequireRoles(
				models.RoleSuperAdmin,
			),
			crewHandler.Create,
		)

		// Solamente SUPER_ADMIN puede listar todas.
		crews.GET(
			"",
			middleware.RequireRoles(
				models.RoleSuperAdmin,
			),
			crewHandler.List,
		)

		// SUPER_ADMIN puede consultar cualquiera.
		// CREW_MANAGER solamente puede consultar la suya.
		crews.GET(
			"/:id",
			middleware.RequireCrewAccess(
				mongodb,
			),
			crewHandler.GetByID,
		)

		// Solamente SUPER_ADMIN puede editar.
		crews.PUT(
			"/:id",
			middleware.RequireRoles(
				models.RoleSuperAdmin,
			),
			crewHandler.Update,
		)

		// Solamente SUPER_ADMIN puede cambiar estados.
		crews.PATCH(
			"/:id/status",
			middleware.RequireRoles(
				models.RoleSuperAdmin,
			),
			crewHandler.UpdateStatus,
		)
	}
}
