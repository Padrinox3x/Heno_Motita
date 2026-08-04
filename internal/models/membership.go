package models

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// MembershipStatus representa el estado de participación
// de un alumno dentro de una cuadrilla.
type MembershipStatus string

const (
	MembershipStatusPending  MembershipStatus = "PENDING"
	MembershipStatusActive   MembershipStatus = "ACTIVE"
	MembershipStatusInactive MembershipStatus = "INACTIVE"
	MembershipStatusExpired  MembershipStatus = "EXPIRED"
	MembershipStatusRevoked  MembershipStatus = "REVOKED"
)

// CrewMembership relaciona un alumno con una cuadrilla.
//
// El usuario permanece en la colección users, mientras que
// esta colección conserva cada periodo de participación.
type CrewMembership struct {
	ID bson.ObjectID `bson:"_id,omitempty" json:"id"`

	CrewID    bson.ObjectID `bson:"crewId" json:"crewId"`
	StudentID bson.ObjectID `bson:"studentId" json:"studentId"`

	ValidFrom  time.Time `bson:"validFrom" json:"validFrom"`
	ValidUntil time.Time `bson:"validUntil" json:"validUntil"`

	Status MembershipStatus `bson:"status" json:"status"`

	// Solo se almacena el hash, nunca el código original.
	ActivationCodeHash string `bson:"activationCodeHash,omitempty" json:"-"`

	ActivationCodeExpiresAt *time.Time `bson:"activationCodeExpiresAt,omitempty" json:"activationCodeExpiresAt,omitempty"`

	ActivatedAt *time.Time `bson:"activatedAt,omitempty" json:"activatedAt,omitempty"`
	ArchivedAt  *time.Time `bson:"archivedAt,omitempty" json:"archivedAt,omitempty"`

	CreatedBy bson.ObjectID `bson:"createdBy" json:"createdBy"`

	CreatedAt time.Time `bson:"createdAt" json:"createdAt"`
	UpdatedAt time.Time `bson:"updatedAt" json:"updatedAt"`
}
