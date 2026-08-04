package database

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// MongoDB contiene el cliente y la base de datos que utilizará la API.
type MongoDB struct {
	Client   *mongo.Client
	Database *mongo.Database
}

// Connect crea el cliente de MongoDB Atlas y verifica la conexión.
//
// El cliente debe crearse una sola vez al iniciar la aplicación.
// Después se reutiliza en repositorios, servicios y controladores.
func Connect(
	parentContext context.Context,
	uri string,
	databaseName string,
) (*MongoDB, error) {
	// MongoDB recomienda utilizar Stable API para evitar cambios
	// inesperados cuando Atlas actualice MongoDB Server.
	serverAPI := options.ServerAPI(options.ServerAPIVersion1)

	clientOptions := options.Client().
		ApplyURI(uri).
		SetServerAPIOptions(serverAPI).
		SetServerSelectionTimeout(10 * time.Second)

	client, err := mongo.Connect(clientOptions)
	if err != nil {
		return nil, fmt.Errorf(
			"no se pudo crear el cliente de MongoDB: %w",
			err,
		)
	}

	// mongo.Connect crea el cliente, pero Ping confirma que realmente
	// se puede localizar y autenticar contra el servidor.
	pingContext, cancel := context.WithTimeout(
		parentContext,
		10*time.Second,
	)
	defer cancel()

	if err := client.Ping(pingContext, nil); err != nil {
		disconnectContext, disconnectCancel := context.WithTimeout(
			context.Background(),
			5*time.Second,
		)
		defer disconnectCancel()

		_ = client.Disconnect(disconnectContext)

		return nil, fmt.Errorf(
			"no se pudo conectar con MongoDB Atlas: %w",
			err,
		)
	}

	return &MongoDB{
		Client:   client,
		Database: client.Database(databaseName),
	}, nil
}

// Ping comprueba que MongoDB continúa disponible.
func (mongodb *MongoDB) Ping(parentContext context.Context) error {
	ctx, cancel := context.WithTimeout(
		parentContext,
		5*time.Second,
	)
	defer cancel()

	if err := mongodb.Client.Ping(ctx, nil); err != nil {
		return fmt.Errorf(
			"MongoDB no está disponible: %w",
			err,
		)
	}

	return nil
}

// Collection devuelve una colección de la base de datos configurada.
func (mongodb *MongoDB) Collection(
	name string,
) *mongo.Collection {
	return mongodb.Database.Collection(name)
}

// Disconnect cierra correctamente el cliente de MongoDB.
func (mongodb *MongoDB) Disconnect(
	parentContext context.Context,
) error {
	ctx, cancel := context.WithTimeout(
		parentContext,
		10*time.Second,
	)
	defer cancel()

	if err := mongodb.Client.Disconnect(ctx); err != nil {
		return fmt.Errorf(
			"no se pudo cerrar la conexión de MongoDB: %w",
			err,
		)
	}

	return nil
}
