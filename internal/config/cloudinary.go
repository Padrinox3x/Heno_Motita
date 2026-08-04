package config

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
)

// CloudinarySettings contiene la configuración del
// servicio de almacenamiento de imágenes.
type CloudinarySettings struct {
	CloudName string
	APIKey    string
	APISecret string

	Folder string

	MaxImageSizeMB    int64
	MaxImageSizeBytes int64
}

// LoadCloudinarySettings carga y valida las variables
// de entorno necesarias para Cloudinary.
func LoadCloudinarySettings() (
	CloudinarySettings,
	error,
) {
	cloudName := strings.TrimSpace(
		os.Getenv("CLOUDINARY_CLOUD_NAME"),
	)

	apiKey := strings.TrimSpace(
		os.Getenv("CLOUDINARY_API_KEY"),
	)

	apiSecret := strings.TrimSpace(
		os.Getenv("CLOUDINARY_API_SECRET"),
	)

	folder := strings.Trim(
		strings.TrimSpace(
			os.Getenv("CLOUDINARY_FOLDER"),
		),
		"/",
	)

	if folder == "" {
		folder = "heno-motita"
	}

	maxImageSizeText := strings.TrimSpace(
		os.Getenv("MAX_IMAGE_SIZE_MB"),
	)

	if maxImageSizeText == "" {
		maxImageSizeText = "8"
	}

	maxImageSizeMB, err := strconv.ParseInt(
		maxImageSizeText,
		10,
		64,
	)
	if err != nil || maxImageSizeMB < 1 {
		return CloudinarySettings{},
			fmt.Errorf(
				"MAX_IMAGE_SIZE_MB debe ser un entero mayor que cero",
			)
	}

	if maxImageSizeMB >
		math.MaxInt64/(1024*1024) {
		return CloudinarySettings{},
			fmt.Errorf(
				"MAX_IMAGE_SIZE_MB es demasiado grande",
			)
	}

	if cloudName == "" {
		return CloudinarySettings{},
			fmt.Errorf(
				"CLOUDINARY_CLOUD_NAME es obligatorio",
			)
	}

	if apiKey == "" {
		return CloudinarySettings{},
			fmt.Errorf(
				"CLOUDINARY_API_KEY es obligatorio",
			)
	}

	if apiSecret == "" {
		return CloudinarySettings{},
			fmt.Errorf(
				"CLOUDINARY_API_SECRET es obligatorio",
			)
	}

	return CloudinarySettings{
		CloudName: cloudName,
		APIKey:    apiKey,
		APISecret: apiSecret,
		Folder:    folder,

		MaxImageSizeMB: maxImageSizeMB,

		MaxImageSizeBytes: maxImageSizeMB *
			1024 *
			1024,
	}, nil
}
