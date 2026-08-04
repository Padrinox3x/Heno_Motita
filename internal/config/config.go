package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config concentra las variables necesarias para ejecutar la API.
type Config struct {
	AppEnv        string
	Port          string
	MongoURI      string
	MongoDatabase string
	FrontendURL   string

	// Usuario inicial del sistema.
	SuperAdminName     string
	SuperAdminEmail    string
	SuperAdminPassword string

	// Configuración para los tokens JWT.
	JWTAccessSecret   string
	JWTIssuer         string
	JWTAccessDuration time.Duration
}

// Load carga las variables del archivo .env durante el desarrollo.
//
// En producción, por ejemplo en Render, las variables se obtienen
// directamente de la configuración del servicio.
func Load() (*Config, error) {
	// En Render posiblemente no existirá un archivo .env.
	// Por eso su ausencia no se considera un error fatal.
	_ = godotenv.Load()

	// Obtener la duración del token como texto.
	jwtAccessDurationText := getEnv(
		"JWT_ACCESS_DURATION",
		"15m",
	)

	// Convertir valores como 15m, 1h o 30s a time.Duration.
	jwtAccessDuration, err := time.ParseDuration(
		jwtAccessDurationText,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"JWT_ACCESS_DURATION no tiene un formato válido: %w",
			err,
		)
	}

	cfg := &Config{
		AppEnv: getEnv(
			"APP_ENV",
			"development",
		),

		Port: getEnv(
			"PORT",
			"8080",
		),

		MongoURI: strings.TrimSpace(
			os.Getenv("MONGODB_URI"),
		),

		MongoDatabase: getEnv(
			"MONGODB_DATABASE",
			"heno_motita",
		),

		FrontendURL: getEnv(
			"FRONTEND_URL",
			"http://localhost:5173",
		),

		SuperAdminName: strings.TrimSpace(
			os.Getenv("SUPER_ADMIN_NAME"),
		),

		SuperAdminEmail: strings.ToLower(
			strings.TrimSpace(
				os.Getenv("SUPER_ADMIN_EMAIL"),
			),
		),

		SuperAdminPassword: strings.TrimSpace(
			os.Getenv("SUPER_ADMIN_PASSWORD"),
		),

		JWTAccessSecret: strings.TrimSpace(
			os.Getenv("JWT_ACCESS_SECRET"),
		),

		JWTIssuer: getEnv(
			"JWT_ISSUER",
			"heno-motita-api",
		),

		JWTAccessDuration: jwtAccessDuration,
	}

	if cfg.MongoURI == "" {
		return nil, fmt.Errorf(
			"la variable de entorno MONGODB_URI es obligatoria",
		)
	}

	if cfg.MongoDatabase == "" {
		return nil, fmt.Errorf(
			"la variable de entorno MONGODB_DATABASE es obligatoria",
		)
	}

	if cfg.SuperAdminName == "" {
		return nil, fmt.Errorf(
			"la variable SUPER_ADMIN_NAME es obligatoria",
		)
	}

	if cfg.SuperAdminEmail == "" {
		return nil, fmt.Errorf(
			"la variable SUPER_ADMIN_EMAIL es obligatoria",
		)
	}

	if cfg.SuperAdminPassword == "" {
		return nil, fmt.Errorf(
			"la variable SUPER_ADMIN_PASSWORD es obligatoria",
		)
	}

	if len(cfg.SuperAdminPassword) < 8 {
		return nil, fmt.Errorf(
			"SUPER_ADMIN_PASSWORD debe tener al menos 8 caracteres",
		)
	}

	if cfg.JWTAccessSecret == "" {
		return nil, fmt.Errorf(
			"la variable JWT_ACCESS_SECRET es obligatoria",
		)
	}

	if len(cfg.JWTAccessSecret) < 32 {
		return nil, fmt.Errorf(
			"JWT_ACCESS_SECRET debe tener al menos 32 caracteres",
		)
	}

	if cfg.JWTIssuer == "" {
		return nil, fmt.Errorf(
			"la variable JWT_ISSUER es obligatoria",
		)
	}

	if cfg.JWTAccessDuration <= 0 {
		return nil, fmt.Errorf(
			"JWT_ACCESS_DURATION debe ser mayor que cero",
		)
	}

	return cfg, nil
}

// getEnv obtiene una variable de entorno.
//
// Cuando la variable no existe o está vacía,
// devuelve el valor predeterminado.
func getEnv(
	key string,
	defaultValue string,
) string {
	value := strings.TrimSpace(
		os.Getenv(key),
	)

	if value == "" {
		return defaultValue
	}

	return value
}
