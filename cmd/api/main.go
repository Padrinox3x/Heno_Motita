package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"heno-motita-api/internal/config"
	"heno-motita-api/internal/database"
	"heno-motita-api/internal/handlers"
	"heno-motita-api/internal/routes"
	"heno-motita-api/internal/security"
	"heno-motita-api/internal/services"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	// ============================================
	// CARGAR CONFIGURACIÓN PRINCIPAL
	// ============================================

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf(
			"Error de configuración: %v",
			err,
		)
	}

	// ============================================
	// CARGAR CONFIGURACIÓN DE CLOUDINARY
	// ============================================

	cloudinarySettings, err :=
		config.LoadCloudinarySettings()

	if err != nil {
		log.Fatalf(
			"Error en configuración de Cloudinary: %v",
			err,
		)
	}

	log.Printf(
		"Configuración de Cloudinary cargada. Carpeta: %s, límite: %d MB",
		cloudinarySettings.Folder,
		cloudinarySettings.MaxImageSizeMB,
	)

	// ============================================
	// CONECTAR CON MONGODB ATLAS
	// ============================================

	mongodb, err := database.Connect(
		context.Background(),
		cfg.MongoURI,
		cfg.MongoDatabase,
	)
	if err != nil {
		log.Fatalf(
			"Error de MongoDB: %v",
			err,
		)
	}

	// Cerrar la conexión cuando termine la aplicación.
	defer func() {
		ctx, cancel := context.WithTimeout(
			context.Background(),
			10*time.Second,
		)
		defer cancel()

		if err := mongodb.Disconnect(ctx); err != nil {
			log.Printf(
				"Error cerrando MongoDB: %v",
				err,
			)
		}
	}()

	log.Printf(
		"Conexión exitosa con MongoDB Atlas. Base de datos: %s",
		cfg.MongoDatabase,
	)

	// ============================================
	// CREAR ÍNDICES DE MONGODB
	// ============================================

	indexContext, indexCancel :=
		context.WithTimeout(
			context.Background(),
			30*time.Second,
		)

	if err := database.EnsureIndexes(
		indexContext,
		mongodb,
	); err != nil {
		indexCancel()

		log.Fatalf(
			"Error creando índices: %v",
			err,
		)
	}

	indexCancel()

	log.Println(
		"Índices de MongoDB configurados correctamente",
	)

	// ============================================
	// CREAR SUPERADMINISTRADOR INICIAL
	// ============================================

	seedContext, seedCancel :=
		context.WithTimeout(
			context.Background(),
			15*time.Second,
		)

	if err := database.SeedSuperAdmin(
		seedContext,
		mongodb,
		cfg.SuperAdminName,
		cfg.SuperAdminEmail,
		cfg.SuperAdminPassword,
	); err != nil {
		seedCancel()

		log.Fatalf(
			"Error creando el superadministrador: %v",
			err,
		)
	}

	seedCancel()

	log.Println(
		"Superadministrador inicial verificado correctamente",
	)

	// ============================================
	// CONFIGURAR JWT
	// ============================================

	jwtManager, err := security.NewJWTManager(
		cfg.JWTAccessSecret,
		cfg.JWTIssuer,
		cfg.JWTAccessDuration,
	)
	if err != nil {
		log.Fatalf(
			"Error configurando JWT: %v",
			err,
		)
	}

	log.Println(
		"Configuración JWT cargada correctamente",
	)

	// ============================================
	// CONFIGURAR CLOUDINARY
	// ============================================

	cloudinaryService, err :=
		services.NewCloudinaryService(
			cloudinarySettings.CloudName,
			cloudinarySettings.APIKey,
			cloudinarySettings.APISecret,
			cloudinarySettings.Folder,
		)

	if err != nil {
		log.Fatalf(
			"No se pudo iniciar Cloudinary: %v",
			err,
		)
	}

	log.Println(
		"Servicio de Cloudinary configurado correctamente",
	)

	// ============================================
	// CREAR CONTROLADORES
	// ============================================

	authHandler := handlers.NewAuthHandler(
		mongodb,
		jwtManager,
	)

	managerHandler := handlers.NewManagerHandler(
		mongodb,
	)

	managerPortalHandler :=
		handlers.NewManagerPortalHandler(
			mongodb,
		)

	crewHandler := handlers.NewCrewHandler(
		mongodb,
	)

	studentHandler := handlers.NewStudentHandler(
		mongodb,
	)

	studentPortalHandler :=
		handlers.NewStudentPortalHandler(
			mongodb,
		)

	studentHistoryHandler :=
		handlers.NewStudentHistoryHandler(
			mongodb,
		)

	treeHandler := handlers.NewTreeHandler(
		mongodb,
	)

	observationHandler :=
		handlers.NewObservationHandler(
			mongodb,
		)

	observationImageHandler :=
		handlers.NewObservationImageHandler(
			mongodb,
			cloudinaryService,
			cloudinarySettings.MaxImageSizeBytes,
		)
	// ============================================
	// CONFIGURAR GIN
	// ============================================

	if cfg.AppEnv == "production" {
		gin.SetMode(
			gin.ReleaseMode,
		)
	}

	router := gin.Default()

	// Controla la memoria utilizada al procesar
	// formularios multipart.
	router.MaxMultipartMemory =
		cloudinarySettings.MaxImageSizeBytes

	// ============================================
	// CONFIGURAR CORS
	// ============================================

	allowedOrigins := loadAllowedOrigins()

	corsConfig := cors.Config{
		AllowOrigins: allowedOrigins,

		AllowMethods: []string{
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
			http.MethodPatch,
			http.MethodDelete,
			http.MethodOptions,
		},

		AllowHeaders: []string{
			"Origin",
			"Content-Type",
			"Accept",
			"Authorization",
			"X-Requested-With",
		},

		ExposeHeaders: []string{
			"Content-Length",
		},

		// Los JWT se envían mediante el encabezado
		// Authorization, no mediante cookies.
		AllowCredentials: false,

		MaxAge: 12 * time.Hour,
	}

	// Permite usar CORS_ALLOWED_ORIGINS=*
	// únicamente cuando se requiera durante desarrollo.
	if len(allowedOrigins) == 1 &&
		allowedOrigins[0] == "*" {
		corsConfig.AllowAllOrigins = true
		corsConfig.AllowOrigins = nil
	}

	router.Use(
		cors.New(corsConfig),
	)

	log.Printf(
		"Orígenes permitidos por CORS: %s",
		strings.Join(allowedOrigins, ", "),
	)

	// ============================================
	// RUTAS DE SALUD
	// ============================================

	router.GET(
		"/health",
		func(c *gin.Context) {
			c.JSON(
				http.StatusOK,
				gin.H{
					"status":      "ok",
					"message":     "API de Heno Motita funcionando",
					"environment": cfg.AppEnv,
				},
			)
		},
	)

	router.GET(
		"/health/database",
		func(c *gin.Context) {
			pingContext, pingCancel :=
				context.WithTimeout(
					c.Request.Context(),
					5*time.Second,
				)
			defer pingCancel()

			if err := mongodb.Ping(
				pingContext,
			); err != nil {
				c.JSON(
					http.StatusServiceUnavailable,
					gin.H{
						"status":  "error",
						"service": "mongodb",
						"message": "No fue posible conectar con MongoDB",
					},
				)
				return
			}

			c.JSON(
				http.StatusOK,
				gin.H{
					"status":   "ok",
					"service":  "mongodb",
					"database": cfg.MongoDatabase,
					"message":  "Conexión con MongoDB Atlas activa",
				},
			)
		},
	)

	// ============================================
	// REGISTRAR RUTAS DE LA API
	// ============================================

	routes.RegisterAuthRoutes(
		router,
		authHandler,
		jwtManager,
		mongodb,
	)

	routes.RegisterManagerRoutes(
		router,
		managerHandler,
		jwtManager,
		mongodb,
	)

	routes.RegisterManagerPortalRoutes(
		router,
		managerPortalHandler,
		jwtManager,
		mongodb,
	)

	routes.RegisterCrewRoutes(
		router,
		crewHandler,
		jwtManager,
		mongodb,
	)

	// Se registra primero el historial porque contiene
	// la ruta estática /students/history.
	routes.RegisterStudentHistoryRoutes(
		router,
		studentHistoryHandler,
		jwtManager,
		mongodb,
	)

	routes.RegisterStudentRoutes(
		router,
		studentHandler,
		jwtManager,
		mongodb,
	)

	routes.RegisterStudentPortalRoutes(
		router,
		studentPortalHandler,
		jwtManager,
		mongodb,
	)

	routes.RegisterTreeRoutes(
		router,
		treeHandler,
		jwtManager,
		mongodb,
	)

	routes.RegisterObservationRoutes(
		router,
		observationHandler,
		jwtManager,
		mongodb,
	)

	routes.RegisterObservationImageRoutes(
		router,
		observationImageHandler,
		jwtManager,
		mongodb,
	)

	// ============================================
	// MANEJO DE RUTAS NO ENCONTRADAS
	// ============================================

	router.NoRoute(
		func(c *gin.Context) {
			c.JSON(
				http.StatusNotFound,
				gin.H{
					"status":  "error",
					"message": "La ruta solicitada no existe",
					"path":    c.Request.URL.Path,
				},
			)
		},
	)

	// ============================================
	// INICIAR SERVIDOR
	// ============================================

	port := strings.TrimSpace(
		cfg.Port,
	)

	// Render proporciona PORT automáticamente.
	if renderPort := strings.TrimSpace(
		os.Getenv("PORT"),
	); renderPort != "" {
		port = renderPort
	}

	if port == "" {
		port = "8080"
	}

	address := "0.0.0.0:" + port

	log.Printf(
		"Servidor iniciado en el puerto %s",
		port,
	)

	log.Println(
		"Rutas principales disponibles:",
	)

	log.Println(
		"GET    /health",
	)

	log.Println(
		"GET    /health/database",
	)

	log.Println(
		"POST   /api/v1/auth/login",
	)

	log.Println(
		"GET    /api/v1/auth/me",
	)

	log.Println(
		"POST   /api/v1/crews/:id/students/batch",
	)

	log.Println(
		"GET    /api/v1/students/history",
	)

	log.Println(
		"GET    /api/v1/students/:id/memberships",
	)

	log.Println(
		"POST   /api/v1/crews/:id/students/:studentId/reactivate",
	)

	log.Println(
		"POST   /api/v1/crews/:id/trees",
	)

	log.Println(
		"GET    /api/v1/crews/:id/trees",
	)

	log.Println(
		"POST   /api/v1/trees/:id/observations",
	)

	log.Println(
		"GET    /api/v1/trees/:id/observations",
	)

	log.Println(
		"POST   /api/v1/observations/:id/images",
	)

	log.Println(
		"GET    /api/v1/observations/:id/images",
	)

	log.Println(
		"DELETE /api/v1/observations/:id/images/:imageId",
	)

	if err := router.Run(address); err != nil {
		log.Fatalf(
			"No se pudo iniciar el servidor: %v",
			err,
		)
	}
}

// loadAllowedOrigins obtiene los dominios permitidos
// desde CORS_ALLOWED_ORIGINS.
//
// Ejemplo:
//
// CORS_ALLOWED_ORIGINS=http://localhost:5173,https://mi-frontend.onrender.com
func loadAllowedOrigins() []string {
	defaultOrigins := []string{
		"http://localhost:5173",
		"http://localhost:3000",
	}

	value := strings.TrimSpace(
		os.Getenv("CORS_ALLOWED_ORIGINS"),
	)

	if value == "" {
		return defaultOrigins
	}

	origins := make(
		[]string,
		0,
	)

	seen := make(
		map[string]struct{},
	)

	for _, origin := range strings.Split(
		value,
		",",
	) {
		origin = strings.TrimSpace(origin)
		origin = strings.TrimSuffix(origin, "/")

		if origin == "" {
			continue
		}

		if _, exists := seen[origin]; exists {
			continue
		}

		seen[origin] = struct{}{}

		origins = append(
			origins,
			origin,
		)
	}

	if len(origins) == 0 {
		return defaultOrigins
	}

	return origins
}
