package routes

import (
	"heno-motita-api/internal/database"
	"heno-motita-api/internal/handlers"
	"heno-motita-api/internal/middleware"
	"heno-motita-api/internal/models"
	"heno-motita-api/internal/security"

	"github.com/gin-gonic/gin"
)

// RegisterObservationImageRoutes registra las rutas
// de fotografías de observaciones.
func RegisterObservationImageRoutes(
	router *gin.Engine,
	handler *handlers.ObservationImageHandler,
	jwtManager *security.JWTManager,
	mongodb *database.MongoDB,
) {
	images := router.Group(
		"/api/v1/observations",
	)

	images.Use(
		middleware.RequireAuth(
			jwtManager,
			mongodb,
		),
	)

	{
		// El alumno solamente puede subir imágenes
		// a observaciones creadas por él.
		images.POST(
			"/:id/images",
			middleware.RequireRoles(
				models.RoleSuperAdmin,
				models.RoleManager,
				models.RoleStudent,
			),
			middleware.RequireObservationWriteAccess(
				mongodb,
			),
			handler.Upload,
		)

		// Los usuarios con acceso a la cuadrilla
		// pueden consultar la evidencia.
		images.GET(
			"/:id/images",
			middleware.RequireRoles(
				models.RoleSuperAdmin,
				models.RoleManager,
				models.RoleStudent,
			),
			middleware.RequireObservationAccess(
				mongodb,
			),
			handler.List,
		)

		images.DELETE(
			"/:id/images/:imageId",
			middleware.RequireRoles(
				models.RoleSuperAdmin,
				models.RoleManager,
				models.RoleStudent,
			),
			middleware.RequireObservationWriteAccess(
				mongodb,
			),
			handler.Delete,
		)
	}
}
