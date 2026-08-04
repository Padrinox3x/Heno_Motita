package routes

import (
	"heno-motita-api/internal/database"
	"heno-motita-api/internal/handlers"
	"heno-motita-api/internal/middleware"
	"heno-motita-api/internal/models"
	"heno-motita-api/internal/security"

	"github.com/gin-gonic/gin"
)

// RegisterTreeRoutes registra las rutas
// del módulo de árboles.
func RegisterTreeRoutes(
	router *gin.Engine,
	treeHandler *handlers.TreeHandler,
	jwtManager *security.JWTManager,
	mongodb *database.MongoDB,
) {
	crewTrees := router.Group(
		"/api/v1/crews",
	)

	crewTrees.Use(
		middleware.RequireAuth(
			jwtManager,
			mongodb,
		),
	)

	{
		// IMPORTANTE:
		// Se utiliza :id porque las demás rutas de crews
		// ya utilizan ese mismo nombre de parámetro.
		crewTrees.POST(
			"/:id/trees",
			middleware.RequireRoles(
				models.RoleSuperAdmin,
				models.RoleManager,
				models.RoleStudent,
			),
			middleware.RequireCrewTreeAccess(
				mongodb,
			),
			treeHandler.Create,
		)

		crewTrees.GET(
			"/:id/trees",
			middleware.RequireRoles(
				models.RoleSuperAdmin,
				models.RoleManager,
				models.RoleStudent,
			),
			middleware.RequireCrewTreeAccess(
				mongodb,
			),
			treeHandler.ListByCrew,
		)
	}

	trees := router.Group(
		"/api/v1/trees",
	)

	trees.Use(
		middleware.RequireAuth(
			jwtManager,
			mongodb,
		),
	)

	{
		trees.GET(
			"/:id",
			middleware.RequireRoles(
				models.RoleSuperAdmin,
				models.RoleManager,
				models.RoleStudent,
			),
			middleware.RequireTreeAccess(
				mongodb,
			),
			treeHandler.GetByID,
		)

		trees.PUT(
			"/:id",
			middleware.RequireRoles(
				models.RoleSuperAdmin,
				models.RoleManager,
				models.RoleStudent,
			),
			middleware.RequireTreeWriteAccess(
				mongodb,
			),
			treeHandler.Update,
		)

		trees.PATCH(
			"/:id/status",
			middleware.RequireRoles(
				models.RoleSuperAdmin,
				models.RoleManager,
			),
			middleware.RequireTreeWriteAccess(
				mongodb,
			),
			treeHandler.UpdateStatus,
		)
	}
}
