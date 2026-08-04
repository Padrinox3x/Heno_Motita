package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"heno-motita-api/internal/database"
	"heno-motita-api/internal/models"
	"heno-motita-api/internal/security"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

const (
	AuthClaimsKey  = "authClaims"
	CurrentUserKey = "currentUser"
)

// RequireAuth protege una ruta mediante JWT.
//
// Además de validar el token, consulta MongoDB para comprobar
// que el usuario todavía existe y continúa activo.
func RequireAuth(
	jwtManager *security.JWTManager,
	mongodb *database.MongoDB,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString, ok := extractBearerToken(
			c.GetHeader("Authorization"),
		)
		if !ok {
			c.AbortWithStatusJSON(
				http.StatusUnauthorized,
				gin.H{
					"status":  "error",
					"message": "Debes enviar un token Bearer",
				},
			)
			return
		}

		claims, err := jwtManager.ParseAccessToken(
			tokenString,
		)
		if err != nil {
			c.AbortWithStatusJSON(
				http.StatusUnauthorized,
				gin.H{
					"status":  "error",
					"message": "Token inválido o expirado",
				},
			)
			return
		}

		userID, err := bson.ObjectIDFromHex(
			claims.UserID,
		)
		if err != nil {
			c.AbortWithStatusJSON(
				http.StatusUnauthorized,
				gin.H{
					"status":  "error",
					"message": "El token contiene un usuario inválido",
				},
			)
			return
		}

		ctx, cancel := context.WithTimeout(
			c.Request.Context(),
			5*time.Second,
		)
		defer cancel()

		var user models.User

		err = mongodb.Collection("users").
			FindOne(
				ctx,
				bson.M{
					"_id": userID,
				},
			).
			Decode(&user)

		if errors.Is(err, mongo.ErrNoDocuments) {
			c.AbortWithStatusJSON(
				http.StatusUnauthorized,
				gin.H{
					"status":  "error",
					"message": "El usuario del token ya no existe",
				},
			)
			return
		}

		if err != nil {
			c.AbortWithStatusJSON(
				http.StatusInternalServerError,
				gin.H{
					"status":  "error",
					"message": "No fue posible validar la sesión",
				},
			)
			return
		}

		if user.Status != models.StatusActive {
			c.AbortWithStatusJSON(
				http.StatusForbidden,
				gin.H{
					"status":  "error",
					"message": "La cuenta no se encuentra activa",
				},
			)
			return
		}

		c.Set(AuthClaimsKey, claims)
		c.Set(CurrentUserKey, &user)

		c.Next()
	}
}

// GetCurrentUser recupera el usuario validado por el middleware.
func GetCurrentUser(
	c *gin.Context,
) (*models.User, bool) {
	value, exists := c.Get(CurrentUserKey)
	if !exists {
		return nil, false
	}

	user, ok := value.(*models.User)
	if !ok || user == nil {
		return nil, false
	}

	return user, true
}

func extractBearerToken(
	authorizationHeader string,
) (string, bool) {
	parts := strings.Fields(
		strings.TrimSpace(authorizationHeader),
	)

	if len(parts) != 2 {
		return "", false
	}

	if !strings.EqualFold(parts[0], "Bearer") {
		return "", false
	}

	token := strings.TrimSpace(parts[1])

	if token == "" {
		return "", false
	}

	return token, true
}
