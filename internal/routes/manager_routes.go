package routes

import (
	"heno-motita-api/internal/database"
	"heno-motita-api/internal/handlers"
	"heno-motita-api/internal/middleware"
	"heno-motita-api/internal/models"
	"heno-motita-api/internal/security"

	"github.com/gin-gonic/gin"
)

// RegisterManagerRoutes registra las rutas de encargados.
//
// Todas requieren un JWT válido y rol SUPER_ADMIN.
func RegisterManagerRoutes(
	router *gin.Engine,
	managerHandler *handlers.ManagerHandler,
	jwtManager *security.JWTManager,
	mongodb *database.MongoDB,
) {
	managers := router.Group(
		"/api/v1/managers",
	)

	managers.Use(
		middleware.RequireAuth(
			jwtManager,
			mongodb,
		),
		middleware.RequireRoles(
			models.RoleSuperAdmin,
		),
	)

	{
		managers.POST(
			"",
			managerHandler.Create,
		)

		managers.GET(
			"",
			managerHandler.List,
		)

		managers.GET(
			"/:id",
			managerHandler.GetByID,
		)

		managers.PUT(
			"/:id",
			managerHandler.Update,
		)

		managers.PATCH(
			"/:id/status",
			managerHandler.UpdateStatus,
		)
	}
}
