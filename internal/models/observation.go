package models

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// ObservationStatus representa el estado de una observación.
type ObservationStatus string

const (
	ObservationStatusActive   ObservationStatus = "ACTIVE"
	ObservationStatusArchived ObservationStatus = "ARCHIVED"
)

// AssessmentMethod identifica el método utilizado
// para evaluar la afectación del árbol.
type AssessmentMethod string

const (
	AssessmentMethodHawksworth AssessmentMethod = "HAWKSWORTH_0_6"
)

// Observation representa una evaluación realizada
// sobre un árbol.
type Observation struct {
	ID bson.ObjectID `bson:"_id,omitempty" json:"id"`

	TreeID bson.ObjectID `bson:"treeId" json:"treeId"`
	CrewID bson.ObjectID `bson:"crewId" json:"crewId"`

	// Usuario que realizó o registró la observación.
	ObserverID bson.ObjectID `bson:"observerId" json:"observerId"`

	Method AssessmentMethod `bson:"method" json:"method"`

	// Cada tercio acepta valores de 0 a 2.
	LowerThirdScore  int `bson:"lowerThirdScore" json:"lowerThirdScore"`
	MiddleThirdScore int `bson:"middleThirdScore" json:"middleThirdScore"`
	UpperThirdScore  int `bson:"upperThirdScore" json:"upperThirdScore"`

	// Se calcula en el backend.
	// No se recibe directamente desde el frontend.
	TotalScore int `bson:"totalScore" json:"totalScore"`

	Notes string `bson:"notes,omitempty" json:"notes,omitempty"`

	ObservationDate time.Time `bson:"observationDate" json:"observationDate"`

	// Coordenadas opcionales tomadas al momento
	// de realizar la observación.
	Latitude  *float64 `bson:"latitude,omitempty" json:"latitude,omitempty"`
	Longitude *float64 `bson:"longitude,omitempty" json:"longitude,omitempty"`

	Status ObservationStatus `bson:"status" json:"status"`

	ArchivedAt *time.Time `bson:"archivedAt,omitempty" json:"archivedAt,omitempty"`

	CreatedAt time.Time `bson:"createdAt" json:"createdAt"`
	UpdatedAt time.Time `bson:"updatedAt" json:"updatedAt"`
}
