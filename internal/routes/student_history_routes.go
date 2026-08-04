package routes

import (
	"heno-motita-api/internal/database"
	"heno-motita-api/internal/handlers"
	"heno-motita-api/internal/middleware"
	"heno-motita-api/internal/models"
	"heno-motita-api/internal/security"

	"github.com/gin-gonic/gin"
)

// RegisterStudentHistoryRoutes registra las rutas
// del historial y reactivación de alumnos.
func RegisterStudentHistoryRoutes(
	router *gin.Engine,
	handler *handlers.StudentHistoryHandler,
	jwtManager *security.JWTManager,
	mongodb *database.MongoDB,
) {
	students := router.Group(
		"/api/v1/students",
	)

	students.Use(
		middleware.RequireAuth(
			jwtManager,
			mongodb,
		),
	)

	students.Use(
		middleware.RequireRoles(
			models.RoleSuperAdmin,
		),
	)

	{
		students.GET(
			"/history",
			handler.List,
		)

		students.GET(
			"/:id/memberships",
			handler.GetMemberships,
		)
	}

	crews := router.Group(
		"/api/v1/crews",
	)

	crews.Use(
		middleware.RequireAuth(
			jwtManager,
			mongodb,
		),
	)

	crews.Use(
		middleware.RequireRoles(
			models.RoleSuperAdmin,
		),
	)

	{
		// Se utiliza :id para evitar conflictos con
		// las demás rutas de cuadrillas.
		crews.POST(
			"/:id/students/:studentId/reactivate",
			handler.Reactivate,
		)
	}
}
