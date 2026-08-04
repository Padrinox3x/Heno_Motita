package models

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type UserRole string

const (
	RoleSuperAdmin UserRole = "SUPER_ADMIN"
	RoleManager    UserRole = "CREW_MANAGER"
	RoleStudent    UserRole = "STUDENT"
)

type UserStatus string

const (
	StatusActive   UserStatus = "ACTIVE"
	StatusInactive UserStatus = "INACTIVE"
	StatusBlocked  UserStatus = "BLOCKED"
)

// User representa a cualquier usuario del sistema.
type User struct {
	ID bson.ObjectID `bson:"_id,omitempty" json:"id"`

	Name       string `bson:"name" json:"name"`
	Email      string `bson:"email" json:"email"`
	Enrollment string `bson:"enrollment,omitempty" json:"enrollment,omitempty"`

	// Datos adicionales utilizados principalmente por encargados.
	Phone       string `bson:"phone,omitempty" json:"phone,omitempty"`
	Institution string `bson:"institution,omitempty" json:"institution,omitempty"`

	PasswordHash string     `bson:"passwordHash" json:"-"`
	Role         UserRole   `bson:"role" json:"role"`
	Status       UserStatus `bson:"status" json:"status"`

	CreatedAt time.Time `bson:"createdAt" json:"createdAt"`
	UpdatedAt time.Time `bson:"updatedAt" json:"updatedAt"`
}
