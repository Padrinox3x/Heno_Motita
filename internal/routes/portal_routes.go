package routes

import (
	"heno-motita-api/internal/database"
	"heno-motita-api/internal/handlers"
	"heno-motita-api/internal/middleware"
	"heno-motita-api/internal/models"
	"heno-motita-api/internal/security"

	"github.com/gin-gonic/gin"
)

// RegisterManagerPortalRoutes registra las rutas
// personalizadas del encargado.
func RegisterManagerPortalRoutes(
	router *gin.Engine,
	handler *handlers.ManagerPortalHandler,
	jwtManager *security.JWTManager,
	mongodb *database.MongoDB,
) {
	manager := router.Group(
		"/api/v1/manager",
	)

	manager.Use(
		middleware.RequireAuth(
			jwtManager,
			mongodb,
		),
	)

	manager.Use(
		middleware.RequireRoles(
			models.RoleManager,
		),
	)

	{
		manager.GET(
			"/dashboard",
			handler.Dashboard,
		)

		manager.GET(
			"/current-crews",
			handler.CurrentCrews,
		)
	}
}

// RegisterStudentPortalRoutes registra las rutas
// personalizadas del alumno.
func RegisterStudentPortalRoutes(
	router *gin.Engine,
	handler *handlers.StudentPortalHandler,
	jwtManager *security.JWTManager,
	mongodb *database.MongoDB,
) {
	student := router.Group(
		"/api/v1/student",
	)

	student.Use(
		middleware.RequireAuth(
			jwtManager,
			mongodb,
		),
	)

	student.Use(
		middleware.RequireRoles(
			models.RoleStudent,
		),
	)

	{
		student.GET(
			"/profile",
			handler.Profile,
		)

		student.GET(
			"/current-crew",
			handler.CurrentCrew,
		)

		student.GET(
			"/trees",
			handler.Trees,
		)

		student.GET(
			"/observations",
			handler.Observations,
		)
	}
}
