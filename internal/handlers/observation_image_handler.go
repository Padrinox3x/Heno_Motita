package handlers

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"heno-motita-api/internal/database"
	"heno-motita-api/internal/dto"
	"heno-motita-api/internal/middleware"
	"heno-motita-api/internal/models"
	"heno-motita-api/internal/services"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const observationImageDescriptionMaxLength = 500

var allowedObservationImageMIMETypes = map[string]string{
	"image/jpeg": "jpg",
	"image/png":  "png",
	"image/webp": "webp",
}

// ObservationImageHandler gestiona las fotografías
// relacionadas con observaciones.
type ObservationImageHandler struct {
	images *mongo.Collection
	users  *mongo.Collection

	cloudinary *services.CloudinaryService

	maxImageSizeBytes int64
}

func NewObservationImageHandler(
	mongodb *database.MongoDB,
	cloudinaryService *services.CloudinaryService,
	maxImageSizeBytes int64,
) *ObservationImageHandler {
	return &ObservationImageHandler{
		images: mongodb.Collection(
			"observation_images",
		),
		users: mongodb.Collection("users"),

		cloudinary: cloudinaryService,

		maxImageSizeBytes: maxImageSizeBytes,
	}
}

// Upload procesa:
//
// POST /api/v1/observations/:id/images
//
// Formulario multipart:
//
// image       archivo obligatorio
// description texto opcional
func (handler *ObservationImageHandler) Upload(
	c *gin.Context,
) {
	observation, ok :=
		middleware.GetCurrentObservation(c)

	if !ok {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "No fue posible recuperar la observación",
			},
		)
		return
	}

	currentUser, ok :=
		middleware.GetCurrentUser(c)

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

	if observation.Status ==
		models.ObservationStatusArchived {
		c.JSON(
			http.StatusConflict,
			gin.H{
				"status":  "error",
				"message": "No se pueden agregar imágenes a una observación archivada",
			},
		)
		return
	}

	if handler.cloudinary == nil {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "El servicio de imágenes no está configurado",
			},
		)
		return
	}

	if handler.maxImageSizeBytes < 1 {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "El límite de imágenes no está configurado",
			},
		)
		return
	}

	// Se agregan 2 MB para permitir los encabezados,
	// boundaries y el campo description del multipart.
	requestLimit :=
		handler.maxImageSizeBytes +
			2*1024*1024

	c.Request.Body = http.MaxBytesReader(
		c.Writer,
		c.Request.Body,
		requestLimit,
	)

	fileHeader, err := c.FormFile("image")
	if err != nil {
		var maxBytesError *http.MaxBytesError

		if errors.As(
			err,
			&maxBytesError,
		) || strings.Contains(
			strings.ToLower(err.Error()),
			"request body too large",
		) {
			c.JSON(
				http.StatusRequestEntityTooLarge,
				gin.H{
					"status":  "error",
					"message": "La imagen supera el tamaño máximo permitido",
				},
			)
			return
		}

		c.JSON(
			http.StatusBadRequest,
			gin.H{
				"status":  "error",
				"message": "Debes enviar una imagen en el campo image",
				"details": err.Error(),
			},
		)
		return
	}

	if fileHeader.Size < 1 {
		c.JSON(
			http.StatusBadRequest,
			gin.H{
				"status":  "error",
				"message": "La imagen está vacía",
			},
		)
		return
	}

	if fileHeader.Size >
		handler.maxImageSizeBytes {
		c.JSON(
			http.StatusRequestEntityTooLarge,
			gin.H{
				"status":        "error",
				"message":       "La imagen supera el tamaño máximo permitido",
				"maximumBytes":  handler.maxImageSizeBytes,
				"receivedBytes": fileHeader.Size,
			},
		)
		return
	}

	description := strings.TrimSpace(
		c.PostForm("description"),
	)

	if len(description) >
		observationImageDescriptionMaxLength {
		c.JSON(
			http.StatusBadRequest,
			gin.H{
				"status":  "error",
				"message": "La descripción no puede superar 500 caracteres",
			},
		)
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		c.JSON(
			http.StatusBadRequest,
			gin.H{
				"status":  "error",
				"message": "No fue posible abrir la imagen",
			},
		)
		return
	}
	defer file.Close()

	mimeType, err :=
		detectObservationImageMIME(file)

	if err != nil {
		c.JSON(
			http.StatusBadRequest,
			gin.H{
				"status":  "error",
				"message": err.Error(),
			},
		)
		return
	}

	originalFilename := sanitizeOriginalFilename(
		fileHeader.Filename,
	)

	uploadContext, uploadCancel :=
		context.WithTimeout(
			c.Request.Context(),
			60*time.Second,
		)
	defer uploadCancel()

	uploadedImage, err :=
		handler.cloudinary.UploadObservationImage(
			uploadContext,
			file,
			observation.ID.Hex(),
			originalFilename,
		)

	if err != nil {
		c.JSON(
			http.StatusBadGateway,
			gin.H{
				"status":  "error",
				"message": "No fue posible subir la imagen",
				"details": err.Error(),
			},
		)
		return
	}

	if uploadedImage.ResourceType != "" &&
		uploadedImage.ResourceType != "image" {
		handler.deleteCloudinaryImageBestEffort(
			uploadedImage.PublicID,
		)

		c.JSON(
			http.StatusUnsupportedMediaType,
			gin.H{
				"status":  "error",
				"message": "Cloudinary no reconoció el archivo como una imagen",
			},
		)
		return
	}

	now := time.Now().UTC()

	image := models.ObservationImage{
		ID: bson.NewObjectID(),

		ObservationID: observation.ID,
		TreeID:        observation.TreeID,
		CrewID:        observation.CrewID,

		UploadedBy: currentUser.ID,

		AssetID:  uploadedImage.AssetID,
		PublicID: uploadedImage.PublicID,

		URL:       uploadedImage.URL,
		SecureURL: uploadedImage.SecureURL,

		OriginalFilename: originalFilename,

		Format: normalizeCloudinaryFormat(
			uploadedImage.Format,
		),
		MimeType: mimeType,

		Width:  uploadedImage.Width,
		Height: uploadedImage.Height,
		Bytes:  int64(uploadedImage.Bytes),

		Description: description,
		CreatedAt:   now,
	}

	ctx, cancel := context.WithTimeout(
		c.Request.Context(),
		10*time.Second,
	)
	defer cancel()

	_, err = handler.images.InsertOne(
		ctx,
		image,
	)
	if err != nil {
		// Si MongoDB falla, se elimina la imagen subida
		// para evitar archivos huérfanos en Cloudinary.
		handler.deleteCloudinaryImageBestEffort(
			uploadedImage.PublicID,
		)

		if mongo.IsDuplicateKeyError(err) {
			c.JSON(
				http.StatusConflict,
				gin.H{
					"status":  "error",
					"message": "La imagen ya se encuentra registrada",
				},
			)
			return
		}

		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "La imagen se subió, pero no pudo registrarse",
			},
		)
		return
	}

	c.JSON(
		http.StatusCreated,
		gin.H{
			"message": "Imagen registrada correctamente",
			"image": dto.NewObservationImageResponse(
				image,
				currentUser,
			),
		},
	)
}

// List procesa:
//
// GET /api/v1/observations/:id/images
//
// Parámetros:
//
// ?page=1
// ?limit=10
func (handler *ObservationImageHandler) List(
	c *gin.Context,
) {
	observation, ok :=
		middleware.GetCurrentObservation(c)

	if !ok {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "No fue posible recuperar la observación",
			},
		)
		return
	}

	page, valid :=
		parseObservationImagePositiveInteger(
			c.Query("page"),
			1,
		)

	if !valid {
		c.JSON(
			http.StatusBadRequest,
			gin.H{
				"status":  "error",
				"message": "page debe ser un entero mayor que cero",
			},
		)
		return
	}

	limit, valid :=
		parseObservationImagePositiveInteger(
			c.Query("limit"),
			10,
		)

	if !valid || limit > 100 {
		c.JSON(
			http.StatusBadRequest,
			gin.H{
				"status":  "error",
				"message": "limit debe estar entre 1 y 100",
			},
		)
		return
	}

	filter := bson.M{
		"observationId": observation.ID,
	}

	ctx, cancel := context.WithTimeout(
		c.Request.Context(),
		12*time.Second,
	)
	defer cancel()

	total, err := handler.images.CountDocuments(
		ctx,
		filter,
	)
	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "No fue posible contar las imágenes",
			},
		)
		return
	}

	skip := int64(
		(page - 1) * limit,
	)

	cursor, err := handler.images.Find(
		ctx,
		filter,
		options.Find().
			SetSort(
				bson.D{
					{
						Key:   "createdAt",
						Value: -1,
					},
				},
			).
			SetSkip(skip).
			SetLimit(int64(limit)),
	)
	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "No fue posible consultar las imágenes",
			},
		)
		return
	}
	defer cursor.Close(ctx)

	images := make(
		[]models.ObservationImage,
		0,
	)

	if err := cursor.All(
		ctx,
		&images,
	); err != nil {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "No fue posible procesar las imágenes",
			},
		)
		return
	}

	uploaders, err :=
		handler.loadObservationImageUploaders(
			ctx,
			images,
		)

	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "No fue posible recuperar los usuarios",
			},
		)
		return
	}

	responses := make(
		[]dto.ObservationImageResponse,
		0,
		len(images),
	)

	for _, image := range images {
		var uploaderPointer *models.User

		uploaderUser, exists :=
			uploaders[image.UploadedBy]

		if exists {
			uploaderCopy := uploaderUser
			uploaderPointer = &uploaderCopy
		}

		responses = append(
			responses,
			dto.NewObservationImageResponse(
				image,
				uploaderPointer,
			),
		)
	}

	totalPages := int64(0)

	if total > 0 {
		totalPages = (total + int64(limit) - 1) / int64(limit)
	}

	c.JSON(
		http.StatusOK,
		dto.ObservationImageListResponse{
			Images: responses,
			Pagination: dto.PaginationResponse{
				Page:       page,
				Limit:      limit,
				Total:      total,
				TotalPages: totalPages,
			},
		},
	)
}

// Delete procesa:
//
// DELETE /api/v1/observations/:id/images/:imageId
func (handler *ObservationImageHandler) Delete(
	c *gin.Context,
) {
	observation, ok :=
		middleware.GetCurrentObservation(c)

	if !ok {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "No fue posible recuperar la observación",
			},
		)
		return
	}

	currentUser, ok :=
		middleware.GetCurrentUser(c)

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

	if observation.Status ==
		models.ObservationStatusArchived {
		c.JSON(
			http.StatusConflict,
			gin.H{
				"status":  "error",
				"message": "No se pueden eliminar imágenes de una observación archivada",
			},
		)
		return
	}

	imageIDText := strings.TrimSpace(
		c.Param("imageId"),
	)

	imageID, err := bson.ObjectIDFromHex(
		imageIDText,
	)
	if err != nil {
		c.JSON(
			http.StatusBadRequest,
			gin.H{
				"status":  "error",
				"message": "El identificador de la imagen no es válido",
			},
		)
		return
	}

	ctx, cancel := context.WithTimeout(
		c.Request.Context(),
		10*time.Second,
	)
	defer cancel()

	var image models.ObservationImage

	err = handler.images.FindOne(
		ctx,
		bson.M{
			"_id":           imageID,
			"observationId": observation.ID,
		},
	).Decode(&image)

	if errors.Is(err, mongo.ErrNoDocuments) {
		c.JSON(
			http.StatusNotFound,
			gin.H{
				"status":  "error",
				"message": "La imagen no existe en esta observación",
			},
		)
		return
	}

	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "No fue posible consultar la imagen",
			},
		)
		return
	}

	// Un alumno solamente puede eliminar fotografías
	// que él mismo haya subido.
	if currentUser.Role == models.RoleStudent &&
		image.UploadedBy != currentUser.ID {
		c.JSON(
			http.StatusForbidden,
			gin.H{
				"status":  "error",
				"message": "Solo puedes eliminar las imágenes que subiste",
			},
		)
		return
	}

	deleteContext, deleteCancel :=
		context.WithTimeout(
			c.Request.Context(),
			30*time.Second,
		)
	defer deleteCancel()

	err = handler.cloudinary.DeleteImage(
		deleteContext,
		image.PublicID,
	)

	if err != nil {
		c.JSON(
			http.StatusBadGateway,
			gin.H{
				"status":  "error",
				"message": "No fue posible eliminar la imagen de Cloudinary",
				"details": err.Error(),
			},
		)
		return
	}

	result, err := handler.images.DeleteOne(
		ctx,
		bson.M{
			"_id":           image.ID,
			"observationId": observation.ID,
		},
	)
	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"status":  "error",
				"message": "La imagen se eliminó de Cloudinary, pero no pudo eliminarse de MongoDB",
			},
		)
		return
	}

	if result.DeletedCount == 0 {
		c.JSON(
			http.StatusNotFound,
			gin.H{
				"status":  "error",
				"message": "La imagen ya no existe",
			},
		)
		return
	}

	c.JSON(
		http.StatusOK,
		gin.H{
			"message": "Imagen eliminada correctamente",
			"imageId": image.ID.Hex(),
		},
	)
}

func detectObservationImageMIME(
	file multipart.File,
) (string, error) {
	buffer := make([]byte, 512)

	bytesRead, err := file.Read(buffer)

	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf(
			"no fue posible inspeccionar la imagen",
		)
	}

	if bytesRead == 0 {
		return "", fmt.Errorf(
			"la imagen está vacía",
		)
	}

	_, err = file.Seek(
		0,
		io.SeekStart,
	)
	if err != nil {
		return "", fmt.Errorf(
			"no fue posible reiniciar la lectura de la imagen",
		)
	}

	mimeType := http.DetectContentType(
		buffer[:bytesRead],
	)

	if _, allowed :=
		allowedObservationImageMIMETypes[mimeType]; !allowed {
		return "", fmt.Errorf(
			"formato no permitido; utiliza JPG, JPEG, PNG o WEBP",
		)
	}

	return mimeType, nil
}

func sanitizeOriginalFilename(
	filename string,
) string {
	filename = strings.ReplaceAll(
		filename,
		"\\",
		"/",
	)

	filename = filepath.Base(filename)
	filename = strings.TrimSpace(filename)

	if filename == "" ||
		filename == "." {
		return "observation-image"
	}

	if len(filename) > 180 {
		extension := filepath.Ext(filename)

		base := strings.TrimSuffix(
			filename,
			extension,
		)

		if len(base) > 150 {
			base = base[:150]
		}

		filename = base + extension
	}

	return filename
}

func normalizeCloudinaryFormat(
	format string,
) string {
	format = strings.ToLower(
		strings.TrimSpace(format),
	)

	if format == "jpeg" {
		return "jpg"
	}

	return format
}

func (
	handler *ObservationImageHandler,
) loadObservationImageUploaders(
	ctx context.Context,
	images []models.ObservationImage,
) (
	map[bson.ObjectID]models.User,
	error,
) {
	result := make(
		map[bson.ObjectID]models.User,
	)

	if len(images) == 0 {
		return result, nil
	}

	userIDs := make(
		[]bson.ObjectID,
		0,
		len(images),
	)

	seen := make(
		map[bson.ObjectID]struct{},
	)

	for _, image := range images {
		if _, exists := seen[image.UploadedBy]; exists {
			continue
		}

		seen[image.UploadedBy] = struct{}{}

		userIDs = append(
			userIDs,
			image.UploadedBy,
		)
	}

	cursor, err := handler.users.Find(
		ctx,
		bson.M{
			"_id": bson.M{
				"$in": userIDs,
			},
		},
	)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var users []models.User

	if err := cursor.All(
		ctx,
		&users,
	); err != nil {
		return nil, err
	}

	for _, user := range users {
		result[user.ID] = user
	}

	return result, nil
}

func (
	handler *ObservationImageHandler,
) deleteCloudinaryImageBestEffort(
	publicID string,
) {
	ctx, cancel := context.WithTimeout(
		context.Background(),
		20*time.Second,
	)
	defer cancel()

	_ = handler.cloudinary.DeleteImage(
		ctx,
		publicID,
	)
}

func parseObservationImagePositiveInteger(
	value string,
	defaultValue int,
) (int, bool) {
	value = strings.TrimSpace(value)

	if value == "" {
		return defaultValue, true
	}

	number, err := strconv.Atoi(value)

	if err != nil || number < 1 {
		return 0, false
	}

	return number, true
}
