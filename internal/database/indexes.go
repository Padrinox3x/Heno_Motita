package database

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// EnsureIndexes crea todos los índices necesarios
// para usuarios, cuadrillas, membresías, árboles,
// observaciones y fotografías.
func EnsureIndexes(
	ctx context.Context,
	mongodb *MongoDB,
) error {
	if err := ensureUserIndexes(
		ctx,
		mongodb,
	); err != nil {
		return err
	}

	if err := ensureCrewIndexes(
		ctx,
		mongodb,
	); err != nil {
		return err
	}

	if err := ensureMembershipIndexes(
		ctx,
		mongodb,
	); err != nil {
		return err
	}

	if err := ensureTreeIndexes(
		ctx,
		mongodb,
	); err != nil {
		return err
	}

	if err := ensureObservationIndexes(
		ctx,
		mongodb,
	); err != nil {
		return err
	}

	if err := ensureObservationImageIndexes(
		ctx,
		mongodb,
	); err != nil {
		return err
	}

	return nil
}

// ==================================================
// ÍNDICES DE USUARIOS
// ==================================================

func ensureUserIndexes(
	ctx context.Context,
	mongodb *MongoDB,
) error {
	usersCollection := mongodb.Collection("users")

	// Evita correos repetidos en todo el sistema.
	emailIndex := mongo.IndexModel{
		Keys: bson.D{
			{
				Key:   "email",
				Value: 1,
			},
		},
		Options: options.Index().
			SetName("users_email_unique").
			SetUnique(true),
	}

	if _, err := usersCollection.
		Indexes().
		CreateOne(
			ctx,
			emailIndex,
		); err != nil {
		return fmt.Errorf(
			"no se pudo crear el índice único de correo: %w",
			err,
		)
	}

	// Evita matrículas repetidas.
	//
	// Sparse permite que administradores y encargados
	// no tengan el campo enrollment.
	enrollmentIndex := mongo.IndexModel{
		Keys: bson.D{
			{
				Key:   "enrollment",
				Value: 1,
			},
		},
		Options: options.Index().
			SetName("users_enrollment_unique").
			SetUnique(true).
			SetSparse(true),
	}

	if _, err := usersCollection.
		Indexes().
		CreateOne(
			ctx,
			enrollmentIndex,
		); err != nil {
		return fmt.Errorf(
			"no se pudo crear el índice único de matrícula: %w",
			err,
		)
	}

	// Consultas de usuarios por rol, estado
	// y fecha de creación.
	roleStatusIndex := mongo.IndexModel{
		Keys: bson.D{
			{
				Key:   "role",
				Value: 1,
			},
			{
				Key:   "status",
				Value: 1,
			},
			{
				Key:   "createdAt",
				Value: -1,
			},
		},
		Options: options.Index().
			SetName(
				"users_role_status_created_at",
			),
	}

	if _, err := usersCollection.
		Indexes().
		CreateOne(
			ctx,
			roleStatusIndex,
		); err != nil {
		return fmt.Errorf(
			"no se pudo crear el índice de usuarios por rol y estado: %w",
			err,
		)
	}

	return nil
}

// ==================================================
// ÍNDICES DE CUADRILLAS
// ==================================================

func ensureCrewIndexes(
	ctx context.Context,
	mongodb *MongoDB,
) error {
	crewsCollection := mongodb.Collection("crews")

	// Consulta de cuadrillas asignadas
	// a un encargado.
	managerStatusIndex := mongo.IndexModel{
		Keys: bson.D{
			{
				Key:   "managerId",
				Value: 1,
			},
			{
				Key:   "status",
				Value: 1,
			},
			{
				Key:   "startAt",
				Value: -1,
			},
		},
		Options: options.Index().
			SetName(
				"crews_manager_status_start_at",
			),
	}

	if _, err := crewsCollection.
		Indexes().
		CreateOne(
			ctx,
			managerStatusIndex,
		); err != nil {
		return fmt.Errorf(
			"no se pudo crear el índice de cuadrillas por encargado: %w",
			err,
		)
	}

	// Consulta de cuadrillas según estado
	// y periodo de vigencia.
	statusDatesIndex := mongo.IndexModel{
		Keys: bson.D{
			{
				Key:   "status",
				Value: 1,
			},
			{
				Key:   "startAt",
				Value: 1,
			},
			{
				Key:   "endAt",
				Value: 1,
			},
		},
		Options: options.Index().
			SetName(
				"crews_status_start_end",
			),
	}

	if _, err := crewsCollection.
		Indexes().
		CreateOne(
			ctx,
			statusDatesIndex,
		); err != nil {
		return fmt.Errorf(
			"no se pudo crear el índice de vigencia de cuadrillas: %w",
			err,
		)
	}

	// Consulta de cuadrillas por zona.
	zoneStatusIndex := mongo.IndexModel{
		Keys: bson.D{
			{
				Key:   "zone",
				Value: 1,
			},
			{
				Key:   "status",
				Value: 1,
			},
		},
		Options: options.Index().
			SetName(
				"crews_zone_status",
			),
	}

	if _, err := crewsCollection.
		Indexes().
		CreateOne(
			ctx,
			zoneStatusIndex,
		); err != nil {
		return fmt.Errorf(
			"no se pudo crear el índice de cuadrillas por zona: %w",
			err,
		)
	}

	// Ordenamiento de cuadrillas por fecha de creación.
	createdAtIndex := mongo.IndexModel{
		Keys: bson.D{
			{
				Key:   "createdAt",
				Value: -1,
			},
		},
		Options: options.Index().
			SetName(
				"crews_created_at",
			),
	}

	if _, err := crewsCollection.
		Indexes().
		CreateOne(
			ctx,
			createdAtIndex,
		); err != nil {
		return fmt.Errorf(
			"no se pudo crear el índice de fecha de cuadrillas: %w",
			err,
		)
	}

	return nil
}

// ==================================================
// ÍNDICES DE MEMBRESÍAS
// ==================================================

func ensureMembershipIndexes(
	ctx context.Context,
	mongodb *MongoDB,
) error {
	membershipsCollection := mongodb.Collection(
		"crew_memberships",
	)

	// Evita que un alumno se registre dos veces
	// dentro de la misma cuadrilla.
	crewStudentIndex := mongo.IndexModel{
		Keys: bson.D{
			{
				Key:   "crewId",
				Value: 1,
			},
			{
				Key:   "studentId",
				Value: 1,
			},
		},
		Options: options.Index().
			SetName(
				"memberships_crew_student_unique",
			).
			SetUnique(true),
	}

	if _, err := membershipsCollection.
		Indexes().
		CreateOne(
			ctx,
			crewStudentIndex,
		); err != nil {
		return fmt.Errorf(
			"no se pudo crear el índice único de membresías: %w",
			err,
		)
	}

	// Consulta del historial de membresías
	// de un alumno.
	studentStatusIndex := mongo.IndexModel{
		Keys: bson.D{
			{
				Key:   "studentId",
				Value: 1,
			},
			{
				Key:   "status",
				Value: 1,
			},
			{
				Key:   "validUntil",
				Value: -1,
			},
		},
		Options: options.Index().
			SetName(
				"memberships_student_status_valid_until",
			),
	}

	if _, err := membershipsCollection.
		Indexes().
		CreateOne(
			ctx,
			studentStatusIndex,
		); err != nil {
		return fmt.Errorf(
			"no se pudo crear el índice de membresías por alumno: %w",
			err,
		)
	}

	// Consulta de alumnos pertenecientes
	// a una cuadrilla.
	crewStatusIndex := mongo.IndexModel{
		Keys: bson.D{
			{
				Key:   "crewId",
				Value: 1,
			},
			{
				Key:   "status",
				Value: 1,
			},
			{
				Key:   "createdAt",
				Value: -1,
			},
		},
		Options: options.Index().
			SetName(
				"memberships_crew_status_created_at",
			),
	}

	if _, err := membershipsCollection.
		Indexes().
		CreateOne(
			ctx,
			crewStatusIndex,
		); err != nil {
		return fmt.Errorf(
			"no se pudo crear el índice de alumnos por cuadrilla: %w",
			err,
		)
	}

	// Consulta de códigos de activación pendientes.
	activationIndex := mongo.IndexModel{
		Keys: bson.D{
			{
				Key:   "studentId",
				Value: 1,
			},
			{
				Key:   "status",
				Value: 1,
			},
			{
				Key:   "activationCodeExpiresAt",
				Value: 1,
			},
			{
				Key:   "validUntil",
				Value: 1,
			},
		},
		Options: options.Index().
			SetName(
				"memberships_student_activation_expiration",
			).
			SetSparse(true),
	}

	if _, err := membershipsCollection.
		Indexes().
		CreateOne(
			ctx,
			activationIndex,
		); err != nil {
		return fmt.Errorf(
			"no se pudo crear el índice de códigos de activación: %w",
			err,
		)
	}

	// Consulta de membresías vencidas.
	expirationIndex := mongo.IndexModel{
		Keys: bson.D{
			{
				Key:   "status",
				Value: 1,
			},
			{
				Key:   "validUntil",
				Value: 1,
			},
		},
		Options: options.Index().
			SetName(
				"memberships_status_valid_until",
			),
	}

	if _, err := membershipsCollection.
		Indexes().
		CreateOne(
			ctx,
			expirationIndex,
		); err != nil {
		return fmt.Errorf(
			"no se pudo crear el índice de vencimiento de membresías: %w",
			err,
		)
	}

	// Consulta del historial completo de un alumno
	// ordenado por periodo de participación.
	studentValidFromIndex := mongo.IndexModel{
		Keys: bson.D{
			{
				Key:   "studentId",
				Value: 1,
			},
			{
				Key:   "validFrom",
				Value: -1,
			},
			{
				Key:   "createdAt",
				Value: -1,
			},
		},
		Options: options.Index().
			SetName(
				"memberships_student_valid_from",
			),
	}

	if _, err := membershipsCollection.
		Indexes().
		CreateOne(
			ctx,
			studentValidFromIndex,
		); err != nil {
		return fmt.Errorf(
			"no se pudo crear el índice del historial de alumnos: %w",
			err,
		)
	}

	// Consulta de cupo ocupado y membresías vigentes
	// dentro de una cuadrilla.
	crewStatusValidUntilIndex := mongo.IndexModel{
		Keys: bson.D{
			{
				Key:   "crewId",
				Value: 1,
			},
			{
				Key:   "status",
				Value: 1,
			},
			{
				Key:   "validUntil",
				Value: 1,
			},
		},
		Options: options.Index().
			SetName(
				"memberships_crew_status_valid_until",
			),
	}

	if _, err := membershipsCollection.
		Indexes().
		CreateOne(
			ctx,
			crewStatusValidUntilIndex,
		); err != nil {
		return fmt.Errorf(
			"no se pudo crear el índice de cupo de cuadrillas: %w",
			err,
		)
	}

	return nil
}

// ==================================================
// ÍNDICES DE ÁRBOLES
// ==================================================

func ensureTreeIndexes(
	ctx context.Context,
	mongodb *MongoDB,
) error {
	treesCollection := mongodb.Collection("trees")

	// Cada árbol tiene un código único
	// en todo el sistema.
	codeIndex := mongo.IndexModel{
		Keys: bson.D{
			{
				Key:   "code",
				Value: 1,
			},
		},
		Options: options.Index().
			SetName("trees_code_unique").
			SetUnique(true),
	}

	if _, err := treesCollection.
		Indexes().
		CreateOne(
			ctx,
			codeIndex,
		); err != nil {
		return fmt.Errorf(
			"no se pudo crear el índice único de código de árbol: %w",
			err,
		)
	}

	// Listado de árboles pertenecientes
	// a una cuadrilla.
	crewStatusCreatedAtIndex := mongo.IndexModel{
		Keys: bson.D{
			{
				Key:   "crewId",
				Value: 1,
			},
			{
				Key:   "status",
				Value: 1,
			},
			{
				Key:   "createdAt",
				Value: -1,
			},
		},
		Options: options.Index().
			SetName(
				"trees_crew_status_created_at",
			),
	}

	if _, err := treesCollection.
		Indexes().
		CreateOne(
			ctx,
			crewStatusCreatedAtIndex,
		); err != nil {
		return fmt.Errorf(
			"no se pudo crear el índice de árboles por cuadrilla: %w",
			err,
		)
	}

	// Consulta de árboles registrados
	// por un usuario.
	registeredByIndex := mongo.IndexModel{
		Keys: bson.D{
			{
				Key:   "registeredBy",
				Value: 1,
			},
			{
				Key:   "createdAt",
				Value: -1,
			},
		},
		Options: options.Index().
			SetName(
				"trees_registered_by_created_at",
			),
	}

	if _, err := treesCollection.
		Indexes().
		CreateOne(
			ctx,
			registeredByIndex,
		); err != nil {
		return fmt.Errorf(
			"no se pudo crear el índice de árboles por usuario: %w",
			err,
		)
	}

	// Consulta de árboles por cuadrilla
	// y nombre común.
	crewCommonNameIndex := mongo.IndexModel{
		Keys: bson.D{
			{
				Key:   "crewId",
				Value: 1,
			},
			{
				Key:   "commonName",
				Value: 1,
			},
		},
		Options: options.Index().
			SetName(
				"trees_crew_common_name",
			),
	}

	if _, err := treesCollection.
		Indexes().
		CreateOne(
			ctx,
			crewCommonNameIndex,
		); err != nil {
		return fmt.Errorf(
			"no se pudo crear el índice de árboles por nombre: %w",
			err,
		)
	}

	return nil
}

// ==================================================
// ÍNDICES DE OBSERVACIONES HAWKSWORTH
// ==================================================

func ensureObservationIndexes(
	ctx context.Context,
	mongodb *MongoDB,
) error {
	observationsCollection := mongodb.Collection(
		"observations",
	)

	// Historial de observaciones de un árbol,
	// ordenado desde la más reciente.
	treeDateIndex := mongo.IndexModel{
		Keys: bson.D{
			{
				Key:   "treeId",
				Value: 1,
			},
			{
				Key:   "observationDate",
				Value: -1,
			},
		},
		Options: options.Index().
			SetName(
				"observations_tree_date",
			),
	}

	if _, err := observationsCollection.
		Indexes().
		CreateOne(
			ctx,
			treeDateIndex,
		); err != nil {
		return fmt.Errorf(
			"no se pudo crear el índice de observaciones por árbol: %w",
			err,
		)
	}

	// Historial de observaciones realizadas
	// dentro de una cuadrilla.
	crewDateIndex := mongo.IndexModel{
		Keys: bson.D{
			{
				Key:   "crewId",
				Value: 1,
			},
			{
				Key:   "observationDate",
				Value: -1,
			},
		},
		Options: options.Index().
			SetName(
				"observations_crew_date",
			),
	}

	if _, err := observationsCollection.
		Indexes().
		CreateOne(
			ctx,
			crewDateIndex,
		); err != nil {
		return fmt.Errorf(
			"no se pudo crear el índice de observaciones por cuadrilla: %w",
			err,
		)
	}

	// Consulta de observaciones realizadas
	// por un usuario.
	observerDateIndex := mongo.IndexModel{
		Keys: bson.D{
			{
				Key:   "observerId",
				Value: 1,
			},
			{
				Key:   "observationDate",
				Value: -1,
			},
		},
		Options: options.Index().
			SetName(
				"observations_observer_date",
			),
	}

	if _, err := observationsCollection.
		Indexes().
		CreateOne(
			ctx,
			observerDateIndex,
		); err != nil {
		return fmt.Errorf(
			"no se pudo crear el índice de observaciones por usuario: %w",
			err,
		)
	}

	// Listado de observaciones por árbol,
	// estado y fecha.
	treeStatusDateIndex := mongo.IndexModel{
		Keys: bson.D{
			{
				Key:   "treeId",
				Value: 1,
			},
			{
				Key:   "status",
				Value: 1,
			},
			{
				Key:   "observationDate",
				Value: -1,
			},
		},
		Options: options.Index().
			SetName(
				"observations_tree_status_date",
			),
	}

	if _, err := observationsCollection.
		Indexes().
		CreateOne(
			ctx,
			treeStatusDateIndex,
		); err != nil {
		return fmt.Errorf(
			"no se pudo crear el índice de observaciones por estado: %w",
			err,
		)
	}

	// Consulta y análisis de observaciones
	// según la puntuación total Hawksworth.
	totalScoreDateIndex := mongo.IndexModel{
		Keys: bson.D{
			{
				Key:   "totalScore",
				Value: -1,
			},
			{
				Key:   "observationDate",
				Value: -1,
			},
		},
		Options: options.Index().
			SetName(
				"observations_total_score_date",
			),
	}

	if _, err := observationsCollection.
		Indexes().
		CreateOne(
			ctx,
			totalScoreDateIndex,
		); err != nil {
		return fmt.Errorf(
			"no se pudo crear el índice de puntuaciones Hawksworth: %w",
			err,
		)
	}

	// Facilita reportes de puntuaciones
	// por cuadrilla y periodo.
	crewScoreDateIndex := mongo.IndexModel{
		Keys: bson.D{
			{
				Key:   "crewId",
				Value: 1,
			},
			{
				Key:   "totalScore",
				Value: -1,
			},
			{
				Key:   "observationDate",
				Value: -1,
			},
		},
		Options: options.Index().
			SetName(
				"observations_crew_score_date",
			),
	}

	if _, err := observationsCollection.
		Indexes().
		CreateOne(
			ctx,
			crewScoreDateIndex,
		); err != nil {
		return fmt.Errorf(
			"no se pudo crear el índice de puntuaciones por cuadrilla: %w",
			err,
		)
	}

	return nil
}

// ==================================================
// ÍNDICES DE FOTOGRAFÍAS DE OBSERVACIONES
// ==================================================

func ensureObservationImageIndexes(
	ctx context.Context,
	mongodb *MongoDB,
) error {
	imagesCollection := mongodb.Collection(
		"observation_images",
	)

	// Cloudinary genera un publicId único
	// para cada imagen.
	publicIDIndex := mongo.IndexModel{
		Keys: bson.D{
			{
				Key:   "publicId",
				Value: 1,
			},
		},
		Options: options.Index().
			SetName(
				"observation_images_public_id_unique",
			).
			SetUnique(true),
	}

	if _, err := imagesCollection.
		Indexes().
		CreateOne(
			ctx,
			publicIDIndex,
		); err != nil {
		return fmt.Errorf(
			"no se pudo crear el índice único de publicId: %w",
			err,
		)
	}

	// El assetId proporcionado por Cloudinary
	// también debe ser único.
	assetIDIndex := mongo.IndexModel{
		Keys: bson.D{
			{
				Key:   "assetId",
				Value: 1,
			},
		},
		Options: options.Index().
			SetName(
				"observation_images_asset_id_unique",
			).
			SetUnique(true).
			SetSparse(true),
	}

	if _, err := imagesCollection.
		Indexes().
		CreateOne(
			ctx,
			assetIDIndex,
		); err != nil {
		return fmt.Errorf(
			"no se pudo crear el índice único de assetId: %w",
			err,
		)
	}

	// Listado de imágenes pertenecientes
	// a una observación.
	observationCreatedAtIndex := mongo.IndexModel{
		Keys: bson.D{
			{
				Key:   "observationId",
				Value: 1,
			},
			{
				Key:   "createdAt",
				Value: -1,
			},
		},
		Options: options.Index().
			SetName(
				"observation_images_observation_created_at",
			),
	}

	if _, err := imagesCollection.
		Indexes().
		CreateOne(
			ctx,
			observationCreatedAtIndex,
		); err != nil {
		return fmt.Errorf(
			"no se pudo crear el índice de imágenes por observación: %w",
			err,
		)
	}

	// Consulta de imágenes relacionadas
	// con un árbol.
	treeCreatedAtIndex := mongo.IndexModel{
		Keys: bson.D{
			{
				Key:   "treeId",
				Value: 1,
			},
			{
				Key:   "createdAt",
				Value: -1,
			},
		},
		Options: options.Index().
			SetName(
				"observation_images_tree_created_at",
			),
	}

	if _, err := imagesCollection.
		Indexes().
		CreateOne(
			ctx,
			treeCreatedAtIndex,
		); err != nil {
		return fmt.Errorf(
			"no se pudo crear el índice de imágenes por árbol: %w",
			err,
		)
	}

	// Consulta de imágenes relacionadas
	// con una cuadrilla.
	crewCreatedAtIndex := mongo.IndexModel{
		Keys: bson.D{
			{
				Key:   "crewId",
				Value: 1,
			},
			{
				Key:   "createdAt",
				Value: -1,
			},
		},
		Options: options.Index().
			SetName(
				"observation_images_crew_created_at",
			),
	}

	if _, err := imagesCollection.
		Indexes().
		CreateOne(
			ctx,
			crewCreatedAtIndex,
		); err != nil {
		return fmt.Errorf(
			"no se pudo crear el índice de imágenes por cuadrilla: %w",
			err,
		)
	}

	// Consulta de imágenes subidas
	// por un usuario.
	uploadedByCreatedAtIndex := mongo.IndexModel{
		Keys: bson.D{
			{
				Key:   "uploadedBy",
				Value: 1,
			},
			{
				Key:   "createdAt",
				Value: -1,
			},
		},
		Options: options.Index().
			SetName(
				"observation_images_uploaded_by_created_at",
			),
	}

	if _, err := imagesCollection.
		Indexes().
		CreateOne(
			ctx,
			uploadedByCreatedAtIndex,
		); err != nil {
		return fmt.Errorf(
			"no se pudo crear el índice de imágenes por usuario: %w",
			err,
		)
	}

	return nil
}
