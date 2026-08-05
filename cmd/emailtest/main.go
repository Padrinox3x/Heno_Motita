package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"heno-motita-api/internal/services"

	"github.com/joho/godotenv"
)

// Script de prueba: envía un correo de verificación
// usando la configuración SMTP del archivo .env.
//
// Uso:
//
//	go run ./cmd/emailtest
//	go run ./cmd/emailtest destinatario@ejemplo.com
func main() {
	_ = godotenv.Load()

	config := services.SMTPConfig{
		Host:     os.Getenv("SMTP_HOST"),
		Port:     os.Getenv("SMTP_PORT"),
		User:     os.Getenv("SMTP_USER"),
		Password: os.Getenv("SMTP_PASSWORD"),
		From:     os.Getenv("SMTP_FROM"),
		FromName: os.Getenv("SMTP_FROM_NAME"),
	}

	emailService := services.NewEmailService(config)

	if !emailService.IsConfigured() {
		fmt.Println(
			"ERROR: SMTP no está configurado en el archivo .env",
		)
		os.Exit(1)
	}

	recipient := os.Getenv("SMTP_FROM")
	if len(os.Args) > 1 &&
		os.Args[1] != "" {
		recipient = os.Args[1]
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		30*time.Second,
	)
	defer cancel()

	expiresAt := time.Now().UTC().Add(
		7 * 24 * time.Hour,
	)

	fmt.Printf(
		"Enviando correo de prueba a %s vía %s...\n",
		recipient,
		config.Host+":"+config.Port,
	)

	err := emailService.SendActivationCode(
		ctx,
		"Usuario de Prueba",
		recipient,
		"HM-PRUEBA-9999",
		expiresAt,
	)
	if err != nil {
		fmt.Printf(
			"FALLO: %v\n",
			err,
		)
		os.Exit(1)
	}

	fmt.Println(
		"CORREO ENVIADO CORRECTAMENTE. Revisa la bandeja de entrada.",
	)
}
