package models

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// CrewStatus representa el estado operativo de una cuadrilla.
type CrewStatus string

const (
	CrewStatusPending   CrewStatus = "PENDING"
	CrewStatusActive    CrewStatus = "ACTIVE"
	CrewStatusFinished  CrewStatus = "FINISHED"
	CrewStatusCancelled CrewStatus = "CANCELLED"
)

// Crew representa una cuadrilla encargada de realizar
// observaciones y seguimiento de árboles en una zona.
type Crew struct {
	ID bson.ObjectID `bson:"_id,omitempty" json:"id"`

	Name        string `bson:"name" json:"name"`
	Description string `bson:"description,omitempty" json:"description"`

	Zone        string `bson:"zone" json:"zone"`
	Institution string `bson:"institution" json:"institution"`

	// Usuario con rol CREW_MANAGER asignado a la cuadrilla.
	ManagerID bson.ObjectID `bson:"managerId" json:"managerId"`

	StartAt time.Time `bson:"startAt" json:"startAt"`
	EndAt   time.Time `bson:"endAt" json:"endAt"`

	StudentLimit int        `bson:"studentLimit" json:"studentLimit"`
	Status       CrewStatus `bson:"status" json:"status"`

	// Superadministrador que creó la cuadrilla.
	CreatedBy bson.ObjectID `bson:"createdBy" json:"createdBy"`

	FinishedAt  *time.Time `bson:"finishedAt,omitempty" json:"finishedAt,omitempty"`
	CancelledAt *time.Time `bson:"cancelledAt,omitempty" json:"cancelledAt,omitempty"`

	CreatedAt time.Time `bson:"createdAt" json:"createdAt"`
	UpdatedAt time.Time `bson:"updatedAt" json:"updatedAt"`
}
