package routes

import (
	"heno-motita-api/internal/database"
	"heno-motita-api/internal/handlers"
	"heno-motita-api/internal/middleware"
	"heno-motita-api/internal/security"

	"github.com/gin-gonic/gin"
)

// RegisterAuthRoutes registra las rutas de autenticación.
func RegisterAuthRoutes(
	router *gin.Engine,
	authHandler *handlers.AuthHandler,
	jwtManager *security.JWTManager,
	mongodb *database.MongoDB,
) {
	api := router.Group("/api/v1")

	auth := api.Group("/auth")
	{
		auth.POST(
			"/login",
			authHandler.Login,
		)

		auth.GET(
			"/me",
			middleware.RequireAuth(
				jwtManager,
				mongodb,
			),
			authHandler.Me,
		)
	}
}
