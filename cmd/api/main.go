package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"heno-motita-api/internal/config"
	"heno-motita-api/internal/database"
	"heno-motita-api/internal/handlers"
	"heno-motita-api/internal/routes"
	"heno-motita-api/internal/security"
	"heno-motita-api/internal/services"

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

	crewHandler := handlers.NewCrewHandler(
		mongodb,
	)

	studentHandler := handlers.NewStudentHandler(
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

	studentHistoryHandler :=
		handlers.NewStudentHistoryHandler(
			mongodb,
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
	//
	// El handler también utiliza http.MaxBytesReader
	// para impedir archivos mayores al límite.
	router.MaxMultipartMemory =
		cloudinarySettings.MaxImageSizeBytes

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

	routes.RegisterCrewRoutes(
		router,
		crewHandler,
		jwtManager,
		mongodb,
	)

	routes.RegisterStudentRoutes(
		router,
		studentHandler,
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

	routes.RegisterStudentHistoryRoutes(
		router,
		studentHistoryHandler,
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

	address := "0.0.0.0:" + cfg.Port

	log.Printf(
		"Servidor iniciado en http://localhost:%s",
		cfg.Port,
	)

	log.Println(
		"Rutas principales disponibles:",
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
		"POST   /api/v1/crews/:id/trees",
	)

	log.Println(
		"POST   /api/v1/trees/:id/observations",
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
