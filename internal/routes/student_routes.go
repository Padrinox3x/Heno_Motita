package routes

import (
	"heno-motita-api/internal/database"
	"heno-motita-api/internal/handlers"
	"heno-motita-api/internal/middleware"
	"heno-motita-api/internal/models"
	"heno-motita-api/internal/security"

	"github.com/gin-gonic/gin"
)

// RegisterStudentRoutes registra las rutas de alumnos,
// membresías y activación.
func RegisterStudentRoutes(
	router *gin.Engine,
	studentHandler *handlers.StudentHandler,
	jwtManager *security.JWTManager,
	mongodb *database.MongoDB,
) {
	// Activación pública.
	router.POST(
		"/api/v1/auth/activate",
		studentHandler.Activate,
	)

	// Rutas relacionadas con alumnos de una cuadrilla.
	crewStudents := router.Group(
		"/api/v1/crews",
	)

	crewStudents.Use(
		middleware.RequireAuth(
			jwtManager,
			mongodb,
		),
	)

	{
		// Únicamente SUPER_ADMIN registra alumnos.
		crewStudents.POST(
			"/:id/students/batch",
			middleware.RequireRoles(
				models.RoleSuperAdmin,
			),
			middleware.RequireCrewAccess(
				mongodb,
			),
			studentHandler.BatchCreate,
		)

		// SUPER_ADMIN puede consultar cualquier cuadrilla.
		// CREW_MANAGER únicamente puede consultar la suya.
		crewStudents.GET(
			"/:id/students",
			middleware.RequireCrewAccess(
				mongodb,
			),
			studentHandler.ListByCrew,
		)
	}

	students := router.Group(
		"/api/v1/students",
	)

	students.Use(
		middleware.RequireAuth(
			jwtManager,
			mongodb,
		),
	)

	{
		students.GET(
			"/:id",
			middleware.RequireRoles(
				models.RoleSuperAdmin,
				models.RoleManager,
			),
			studentHandler.GetByID,
		)

		students.PUT(
			"/:id",
			middleware.RequireRoles(
				models.RoleSuperAdmin,
			),
			studentHandler.Update,
		)

		students.PATCH(
			"/:id/status",
			middleware.RequireRoles(
				models.RoleSuperAdmin,
			),
			studentHandler.UpdateStatus,
		)

		students.POST(
			"/:id/new-activation-code",
			middleware.RequireRoles(
				models.RoleSuperAdmin,
			),
			studentHandler.NewActivationCode,
		)
	}
}
