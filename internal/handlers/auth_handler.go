package handlers

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"heno-motita-api/internal/database"
	"heno-motita-api/internal/dto"
	"heno-motita-api/internal/middleware"
	"heno-motita-api/internal/models"
	"heno-motita-api/internal/security"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// AuthHandler contiene las dependencias de autenticación.
type AuthHandler struct {
	users      *mongo.Collection
	jwtManager *security.JWTManager
}

// NewAuthHandler crea el controlador de autenticación.
func NewAuthHandler(
	mongodb *database.MongoDB,
	jwtManager *security.JWTManager,
) *AuthHandler {
	return &AuthHandler{
		users:      mongodb.Collection("users"),
		jwtManager: jwtManager,
	}
}

// Login procesa POST /api/v1/auth/login.
func (handler *AuthHandler) Login(
	c *gin.Context,
) {
	var request dto.LoginRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(
			http.StatusBadRequest,
			gin.H{
				"status":  "error",
				"message": "El correo y la contraseña son obligatorios",
				"details": err.Error(),
			},
		)
		return
	}

	email := strings.ToLower(
		strings.TrimSpace(request.Email),
	)

	ctx, cancel := context.WithTimeout(
		c.Request.Context(),
		5*time.Second,
	)
	defer cancel()

	var user models.User

	err := handler.users.FindOne(
		ctx,
		bson.M{
			"email": email,
		},
	).Decode(&user)

	if errors.Is(err, mongo.ErrNoDocuments) {
		loginUnauthorized(c)
		return
	}

	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "No fue posible realizar el inicio de sesión",
			},
		)
		return
	}

	if user.Status != models.StatusActive {
		c.JSON(
			http.StatusForbidden,
			gin.H{
				"status":  "error",
				"message": "La cuenta no se encuentra activa",
			},
		)
		return
	}

	if err := security.CheckPassword(
		user.PasswordHash,
		request.Password,
	); err != nil {
		loginUnauthorized(c)
		return
	}

	accessToken, expiresAt, err :=
		handler.jwtManager.GenerateAccessToken(user)

	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "No fue posible generar la sesión",
			},
		)
		return
	}

	response := dto.LoginResponse{
		Message:     "Inicio de sesión correcto",
		AccessToken: accessToken,
		TokenType:   "Bearer",
		ExpiresIn: int64(
			time.Until(expiresAt).Seconds(),
		),
		ExpiresAt: expiresAt,
		User:      dto.NewUserResponse(user),
	}

	c.JSON(
		http.StatusOK,
		response,
	)
}

// Me procesa GET /api/v1/auth/me.
func (handler *AuthHandler) Me(
	c *gin.Context,
) {
	user, ok := middleware.GetCurrentUser(c)
	if !ok {
		c.JSON(
			http.StatusUnauthorized,
			gin.H{
				"status":  "error",
				"message": "No se encontró una sesión válida",
			},
		)
		return
	}

	c.JSON(
		http.StatusOK,
		gin.H{
			"message": "Usuario autenticado",
			"user":    dto.NewUserResponse(*user),
		},
	)
}

func loginUnauthorized(
	c *gin.Context,
) {
	c.JSON(
		http.StatusUnauthorized,
		gin.H{
			"status":  "error",
			"message": "Correo o contraseña incorrectos",
		},
	)
}
