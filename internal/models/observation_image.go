package models

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// ObservationImage representa una evidencia fotográfica
// relacionada con una observación Hawksworth.
type ObservationImage struct {
	ID bson.ObjectID `bson:"_id,omitempty" json:"id"`

	ObservationID bson.ObjectID `bson:"observationId" json:"observationId"`
	TreeID        bson.ObjectID `bson:"treeId" json:"treeId"`
	CrewID        bson.ObjectID `bson:"crewId" json:"crewId"`

	UploadedBy bson.ObjectID `bson:"uploadedBy" json:"uploadedBy"`

	// Identificadores proporcionados por Cloudinary.
	AssetID  string `bson:"assetId" json:"assetId"`
	PublicID string `bson:"publicId" json:"publicId"`

	URL       string `bson:"url" json:"url"`
	SecureURL string `bson:"secureUrl" json:"secureUrl"`

	OriginalFilename string `bson:"originalFilename" json:"originalFilename"`

	Format   string `bson:"format" json:"format"`
	MimeType string `bson:"mimeType" json:"mimeType"`

	Width  int `bson:"width" json:"width"`
	Height int `bson:"height" json:"height"`

	Bytes int64 `bson:"bytes" json:"bytes"`

	Description string `bson:"description,omitempty" json:"description,omitempty"`

	CreatedAt time.Time `bson:"createdAt" json:"createdAt"`
}
