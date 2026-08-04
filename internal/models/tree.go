package models

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// TreeStatus representa el estado de un árbol
// dentro del sistema.
type TreeStatus string

const (
	TreeStatusActive   TreeStatus = "ACTIVE"
	TreeStatusInactive TreeStatus = "INACTIVE"
	TreeStatusArchived TreeStatus = "ARCHIVED"
)

// Tree representa un árbol registrado dentro
// de una cuadrilla.
type Tree struct {
	ID bson.ObjectID `bson:"_id,omitempty" json:"id"`

	CrewID bson.ObjectID `bson:"crewId" json:"crewId"`

	// Código identificador del árbol.
	// Ejemplo: ARB-TULA-001
	Code string `bson:"code" json:"code"`

	CommonName     string `bson:"commonName" json:"commonName"`
	ScientificName string `bson:"scientificName,omitempty" json:"scientificName,omitempty"`

	Latitude  float64 `bson:"latitude" json:"latitude"`
	Longitude float64 `bson:"longitude" json:"longitude"`

	LocationDescription string `bson:"locationDescription,omitempty" json:"locationDescription,omitempty"`

	Status TreeStatus `bson:"status" json:"status"`

	// Usuario que registró el árbol.
	RegisteredBy bson.ObjectID `bson:"registeredBy" json:"registeredBy"`

	ArchivedAt *time.Time `bson:"archivedAt,omitempty" json:"archivedAt,omitempty"`

	CreatedAt time.Time `bson:"createdAt" json:"createdAt"`
	UpdatedAt time.Time `bson:"updatedAt" json:"updatedAt"`
}
