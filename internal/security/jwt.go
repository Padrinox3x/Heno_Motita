package security

import (
	"fmt"
	"strings"
	"time"

	"heno-motita-api/internal/models"

	"github.com/golang-jwt/jwt/v5"
)

const accessTokenType = "access"

// Claims representa la información almacenada dentro del JWT.
type Claims struct {
	UserID    string          `json:"userId"`
	Email     string          `json:"email"`
	Role      models.UserRole `json:"role"`
	TokenType string          `json:"tokenType"`

	jwt.RegisteredClaims
}

// JWTManager genera y valida tokens de acceso.
type JWTManager struct {
	accessSecret   []byte
	issuer         string
	accessDuration time.Duration
}

// NewJWTManager crea el administrador de JWT.
func NewJWTManager(
	accessSecret string,
	issuer string,
	accessDuration time.Duration,
) (*JWTManager, error) {
	accessSecret = strings.TrimSpace(accessSecret)
	issuer = strings.TrimSpace(issuer)

	if len(accessSecret) < 32 {
		return nil, fmt.Errorf(
			"el secreto JWT debe tener al menos 32 caracteres",
		)
	}

	if issuer == "" {
		return nil, fmt.Errorf(
			"el emisor JWT es obligatorio",
		)
	}

	if accessDuration <= 0 {
		return nil, fmt.Errorf(
			"la duración del JWT debe ser mayor que cero",
		)
	}

	return &JWTManager{
		accessSecret:   []byte(accessSecret),
		issuer:         issuer,
		accessDuration: accessDuration,
	}, nil
}

// GenerateAccessToken genera un JWT para un usuario autenticado.
func (manager *JWTManager) GenerateAccessToken(
	user models.User,
) (
	string,
	time.Time,
	error,
) {
	now := time.Now().UTC()
	expiresAt := now.Add(manager.accessDuration)

	claims := Claims{
		UserID:    user.ID.Hex(),
		Email:     user.Email,
		Role:      user.Role,
		TokenType: accessTokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    manager.issuer,
			Subject:   user.ID.Hex(),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}

	token := jwt.NewWithClaims(
		jwt.SigningMethodHS256,
		claims,
	)

	signedToken, err := token.SignedString(
		manager.accessSecret,
	)
	if err != nil {
		return "", time.Time{}, fmt.Errorf(
			"no se pudo firmar el JWT: %w",
			err,
		)
	}

	return signedToken, expiresAt, nil
}

// ParseAccessToken verifica la firma y las claims del JWT.
func (manager *JWTManager) ParseAccessToken(
	tokenString string,
) (*Claims, error) {
	tokenString = strings.TrimSpace(tokenString)

	if tokenString == "" {
		return nil, fmt.Errorf(
			"el token está vacío",
		)
	}

	claims := &Claims{}

	token, err := jwt.ParseWithClaims(
		tokenString,
		claims,
		func(token *jwt.Token) (any, error) {
			if token.Method.Alg() != jwt.SigningMethodHS256.Alg() {
				return nil, fmt.Errorf(
					"algoritmo JWT no permitido: %s",
					token.Method.Alg(),
				)
			}

			return manager.accessSecret, nil
		},
		jwt.WithValidMethods(
			[]string{
				jwt.SigningMethodHS256.Alg(),
			},
		),
		jwt.WithIssuer(manager.issuer),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithLeeway(5*time.Second),
	)
	if err != nil {
		return nil, fmt.Errorf(
			"token inválido: %w",
			err,
		)
	}

	if !token.Valid {
		return nil, fmt.Errorf(
			"el token no es válido",
		)
	}

	if claims.TokenType != accessTokenType {
		return nil, fmt.Errorf(
			"el token no es de acceso",
		)
	}

	if claims.UserID == "" {
		return nil, fmt.Errorf(
			"el token no contiene el usuario",
		)
	}

	if claims.Subject != claims.UserID {
		return nil, fmt.Errorf(
			"el identificador del token no coincide",
		)
	}

	return claims, nil
}

// AccessDuration devuelve la duración configurada.
func (manager *JWTManager) AccessDuration() time.Duration {
	return manager.accessDuration
}
