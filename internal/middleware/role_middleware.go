package middleware

import (
	"net/http"

	"heno-motita-api/internal/models"

	"github.com/gin-gonic/gin"
)

// RequireRoles verifica que el usuario autenticado tenga
// alguno de los roles permitidos.
func RequireRoles(
	allowedRoles ...models.UserRole,
) gin.HandlerFunc {
	allowed := make(
		map[models.UserRole]struct{},
		len(allowedRoles),
	)

	for _, role := range allowedRoles {
		allowed[role] = struct{}{}
	}

	return func(c *gin.Context) {
		user, ok := GetCurrentUser(c)
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

		if _, exists := allowed[user.Role]; !exists {
			c.AbortWithStatusJSON(
				http.StatusForbidden,
				gin.H{
					"status":  "error",
					"message": "No tienes permisos para realizar esta acción",
				},
			)
			return
		}

		c.Next()
	}
}
