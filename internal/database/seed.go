package database

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"heno-motita-api/internal/models"
	"heno-motita-api/internal/security"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// SeedSuperAdmin crea el primer administrador si todavía no existe.
func SeedSuperAdmin(
	ctx context.Context,
	mongodb *MongoDB,
	name string,
	email string,
	password string,
) error {
	usersCollection := mongodb.Collection("users")

	name = strings.TrimSpace(name)
	email = strings.ToLower(strings.TrimSpace(email))

	if name == "" {
		return fmt.Errorf(
			"el nombre del superadministrador es obligatorio",
		)
	}

	if email == "" {
		return fmt.Errorf(
			"el correo del superadministrador es obligatorio",
		)
	}

	var existingUser models.User

	err := usersCollection.FindOne(
		ctx,
		bson.M{
			"email": email,
		},
	).Decode(&existingUser)

	if err == nil {
		log.Printf(
			"El superadministrador ya existe: %s",
			email,
		)
		return nil
	}

	if !errors.Is(err, mongo.ErrNoDocuments) {
		return fmt.Errorf(
			"no se pudo consultar el superadministrador: %w",
			err,
		)
	}

	passwordHash, err := security.HashPassword(password)
	if err != nil {
		return fmt.Errorf(
			"contraseña del superadministrador inválida: %w",
			err,
		)
	}

	now := time.Now().UTC()

	superAdmin := models.User{
		ID:           bson.NewObjectID(),
		Name:         name,
		Email:        email,
		PasswordHash: passwordHash,
		Role:         models.RoleSuperAdmin,
		Status:       models.StatusActive,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	result, err := usersCollection.InsertOne(
		ctx,
		superAdmin,
	)
	if err != nil {
		return fmt.Errorf(
			"no se pudo crear el superadministrador: %w",
			err,
		)
	}

	log.Printf(
		"Superadministrador creado correctamente. ID: %v",
		result.InsertedID,
	)

	return nil
}
