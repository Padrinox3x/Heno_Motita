package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"github.com/cloudinary/cloudinary-go/v2"
	"github.com/cloudinary/cloudinary-go/v2/api/uploader"
)

// CloudinaryUploadResult contiene únicamente los datos
// necesarios para registrar una imagen en MongoDB.
type CloudinaryUploadResult struct {
	AssetID  string
	PublicID string

	URL       string
	SecureURL string

	Format       string
	ResourceType string

	Width  int
	Height int

	Bytes int

	CreatedAt time.Time
}

// CloudinaryService gestiona la subida y eliminación
// de imágenes.
type CloudinaryService struct {
	client *cloudinary.Cloudinary
	folder string
}

// NewCloudinaryService crea una instancia configurada
// del SDK de Cloudinary.
func NewCloudinaryService(
	cloudName string,
	apiKey string,
	apiSecret string,
	folder string,
) (*CloudinaryService, error) {
	cloudName = strings.TrimSpace(cloudName)
	apiKey = strings.TrimSpace(apiKey)
	apiSecret = strings.TrimSpace(apiSecret)
	folder = strings.Trim(
		strings.TrimSpace(folder),
		"/",
	)

	if cloudName == "" ||
		apiKey == "" ||
		apiSecret == "" {
		return nil, fmt.Errorf(
			"la configuración de Cloudinary está incompleta",
		)
	}

	if folder == "" {
		folder = "heno-motita"
	}

	client, err := cloudinary.NewFromParams(
		cloudName,
		apiKey,
		apiSecret,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"no se pudo configurar Cloudinary: %w",
			err,
		)
	}

	return &CloudinaryService{
		client: client,
		folder: folder,
	}, nil
}

// UploadObservationImage sube una imagen a:
//
// heno-motita/observations/{observationId}
func (service *CloudinaryService) UploadObservationImage(
	ctx context.Context,
	file io.Reader,
	observationID string,
	originalFilename string,
) (*CloudinaryUploadResult, error) {
	if file == nil {
		return nil, fmt.Errorf(
			"el archivo de imagen es obligatorio",
		)
	}

	observationID = strings.TrimSpace(
		observationID,
	)

	if observationID == "" {
		return nil, fmt.Errorf(
			"el identificador de la observación es obligatorio",
		)
	}

	publicID, err := generateCloudinaryPublicID()
	if err != nil {
		return nil, err
	}

	uploadFolder := path.Join(
		service.folder,
		"observations",
		observationID,
	)

	result, err := service.client.Upload.Upload(
		ctx,
		file,
		uploader.UploadParams{
			PublicID:     publicID,
			Folder:       uploadFolder,
			ResourceType: "image",

			// Permite conservar un nombre legible
			// dentro de la biblioteca de Cloudinary.
			FilenameOverride: strings.TrimSpace(
				originalFilename,
			),
		},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"Cloudinary rechazó la imagen: %w",
			err,
		)
	}

	if result == nil ||
		strings.TrimSpace(result.PublicID) == "" ||
		strings.TrimSpace(result.SecureURL) == "" {
		return nil, fmt.Errorf(
			"Cloudinary no devolvió información válida de la imagen",
		)
	}

	return &CloudinaryUploadResult{
		AssetID:      result.AssetID,
		PublicID:     result.PublicID,
		URL:          result.URL,
		SecureURL:    result.SecureURL,
		Format:       strings.ToLower(result.Format),
		ResourceType: result.ResourceType,
		Width:        result.Width,
		Height:       result.Height,
		Bytes:        result.Bytes,
		CreatedAt:    result.CreatedAt,
	}, nil
}

// DeleteImage elimina una imagen de Cloudinary e invalida
// sus copias almacenadas en la CDN.
func (service *CloudinaryService) DeleteImage(
	ctx context.Context,
	publicID string,
) error {
	publicID = strings.TrimSpace(publicID)

	if publicID == "" {
		return fmt.Errorf(
			"el publicId de Cloudinary es obligatorio",
		)
	}

	invalidate := true

	result, err := service.client.Upload.Destroy(
		ctx,
		uploader.DestroyParams{
			PublicID:     publicID,
			ResourceType: "image",
			Invalidate:   &invalidate,
		},
	)
	if err != nil {
		return fmt.Errorf(
			"no se pudo eliminar la imagen de Cloudinary: %w",
			err,
		)
	}

	if result == nil {
		return fmt.Errorf(
			"Cloudinary no devolvió el resultado de eliminación",
		)
	}

	switch strings.ToLower(
		strings.TrimSpace(result.Result),
	) {
	case "ok", "not found":
		return nil

	default:
		return fmt.Errorf(
			"Cloudinary no confirmó la eliminación: %s",
			result.Result,
		)
	}
}

func generateCloudinaryPublicID() (
	string,
	error,
) {
	randomBytes := make([]byte, 16)

	_, err := rand.Read(randomBytes)
	if err != nil {
		return "", fmt.Errorf(
			"no se pudo generar el identificador de la imagen: %w",
			err,
		)
	}

	return "image-" +
			hex.EncodeToString(randomBytes),
		nil
}
