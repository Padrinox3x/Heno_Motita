package routes

import (
	"heno-motita-api/internal/database"
	"heno-motita-api/internal/handlers"
	"heno-motita-api/internal/middleware"
	"heno-motita-api/internal/models"
	"heno-motita-api/internal/security"

	"github.com/gin-gonic/gin"
)

// RegisterObservationRoutes registra las rutas
// del módulo de observaciones.
func RegisterObservationRoutes(
	router *gin.Engine,
	observationHandler *handlers.ObservationHandler,
	jwtManager *security.JWTManager,
	mongodb *database.MongoDB,
) {
	treeObservations := router.Group(
		"/api/v1/trees",
	)

	treeObservations.Use(
		middleware.RequireAuth(
			jwtManager,
			mongodb,
		),
	)

	{
		treeObservations.POST(
			"/:id/observations",
			middleware.RequireRoles(
				models.RoleSuperAdmin,
				models.RoleManager,
				models.RoleStudent,
			),
			middleware.RequireTreeAccess(
				mongodb,
			),
			observationHandler.Create,
		)

		treeObservations.GET(
			"/:id/observations",
			middleware.RequireRoles(
				models.RoleSuperAdmin,
				models.RoleManager,
				models.RoleStudent,
			),
			middleware.RequireTreeAccess(
				mongodb,
			),
			observationHandler.ListByTree,
		)
	}

	observations := router.Group(
		"/api/v1/observations",
	)

	observations.Use(
		middleware.RequireAuth(
			jwtManager,
			mongodb,
		),
	)

	{
		observations.GET(
			"/:id",
			middleware.RequireRoles(
				models.RoleSuperAdmin,
				models.RoleManager,
				models.RoleStudent,
			),
			middleware.RequireObservationAccess(
				mongodb,
			),
			observationHandler.GetByID,
		)

		// El alumno solamente puede modificar
		// observaciones creadas por él.
		observations.PUT(
			"/:id",
			middleware.RequireRoles(
				models.RoleSuperAdmin,
				models.RoleManager,
				models.RoleStudent,
			),
			middleware.RequireObservationWriteAccess(
				mongodb,
			),
			observationHandler.Update,
		)

		// Solamente el administrador o encargado
		// pueden archivar observaciones.
		observations.PATCH(
			"/:id/status",
			middleware.RequireRoles(
				models.RoleSuperAdmin,
				models.RoleManager,
			),
			middleware.RequireObservationWriteAccess(
				mongodb,
			),
			observationHandler.UpdateStatus,
		)
	}
}
