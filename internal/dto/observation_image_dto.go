package dto

import (
	"time"

	"heno-motita-api/internal/models"
)

// ObservationImageUploaderResponse contiene información
// pública del usuario que subió la imagen.
type ObservationImageUploaderResponse struct {
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Email string          `json:"email"`
	Role  models.UserRole `json:"role"`
}

// ObservationImageResponse representa una imagen
// relacionada con una observación.
type ObservationImageResponse struct {
	ID string `json:"id"`

	ObservationID string `json:"observationId"`
	TreeID        string `json:"treeId"`
	CrewID        string `json:"crewId"`

	UploadedByID string                            `json:"uploadedById"`
	UploadedBy   *ObservationImageUploaderResponse `json:"uploadedBy,omitempty"`

	AssetID  string `json:"assetId"`
	PublicID string `json:"publicId"`

	URL       string `json:"url"`
	SecureURL string `json:"secureUrl"`

	OriginalFilename string `json:"originalFilename"`

	Format   string `json:"format"`
	MimeType string `json:"mimeType"`

	Width  int `json:"width"`
	Height int `json:"height"`

	Bytes int64 `json:"bytes"`

	Description string `json:"description,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
}

// ObservationImageListResponse representa una lista
// paginada de imágenes.
type ObservationImageListResponse struct {
	Images     []ObservationImageResponse `json:"images"`
	Pagination PaginationResponse         `json:"pagination"`
}

// NewObservationImageResponse transforma el modelo
// en una respuesta pública.
func NewObservationImageResponse(
	image models.ObservationImage,
	uploader *models.User,
) ObservationImageResponse {
	response := ObservationImageResponse{
		ID:               image.ID.Hex(),
		ObservationID:    image.ObservationID.Hex(),
		TreeID:           image.TreeID.Hex(),
		CrewID:           image.CrewID.Hex(),
		UploadedByID:     image.UploadedBy.Hex(),
		AssetID:          image.AssetID,
		PublicID:         image.PublicID,
		URL:              image.URL,
		SecureURL:        image.SecureURL,
		OriginalFilename: image.OriginalFilename,
		Format:           image.Format,
		MimeType:         image.MimeType,
		Width:            image.Width,
		Height:           image.Height,
		Bytes:            image.Bytes,
		Description:      image.Description,
		CreatedAt:        image.CreatedAt,
	}

	if uploader != nil {
		response.UploadedBy = &ObservationImageUploaderResponse{
			ID:    uploader.ID.Hex(),
			Name:  uploader.Name,
			Email: uploader.Email,
			Role:  uploader.Role,
		}
	}

	return response
}
