package services

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/sendgrid/sendgrid-go"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
)

// SendGridConfig concentra los datos necesarios
// para enviar correos con la API de SendGrid.
type SendGridConfig struct {
	APIKey   string
	From     string
	FromName string
}

// EmailService gestiona el envío de correos
// de activación mediante la API v3 de SendGrid.
type EmailService struct {
	config SendGridConfig
	client *sendgrid.Client
}

// NewEmailService crea el servicio de correo.
//
// Si no se configura SENDGRID_API_KEY, el servicio no envía
// correos reales: imprime en consola el contenido que habría
// enviado (útil en desarrollo).
func NewEmailService(
	config SendGridConfig,
) *EmailService {
	config.APIKey = strings.TrimSpace(config.APIKey)
	config.From = strings.TrimSpace(config.From)
	config.FromName = strings.TrimSpace(config.FromName)

	if config.FromName == "" {
		config.FromName = "Heno Motita"
	}

	service := &EmailService{
		config: config,
	}

	if config.APIKey != "" {
		service.client = sendgrid.NewSendClient(
			config.APIKey,
		)
	}

	return service
}

// IsConfigured indica si hay una API key de SendGrid
// configurada.
func (service *EmailService) IsConfigured() bool {
	return service.client != nil &&
		service.config.From != ""
}

// SendActivationCode envía al alumno el correo
// con su código de activación temporal.
func (service *EmailService) SendActivationCode(
	ctx context.Context,
	name string,
	to string,
	code string,
	expiresAt time.Time,
) error {
	name = strings.TrimSpace(name)
	to = strings.ToLower(strings.TrimSpace(to))
	code = strings.TrimSpace(code)

	if to == "" {
		return fmt.Errorf(
			"el destinatario del correo es obligatorio",
		)
	}

	if code == "" {
		return fmt.Errorf(
			"el código de activación es obligatorio",
		)
	}

	expiresText := expiresAt.UTC().Format(
		"02/01/2006 15:04 UTC",
	)

	subject := "Heno Motita - Tu código de activación"

	textBody := fmt.Sprintf(
		`Hola %s,

Has sido registrado en el programa de monitoreo ambiental Heno Motita.

Para activar tu cuenta utiliza el siguiente código:

    %s

El código es válido hasta el %s.
Si no lo usas antes de esa fecha, necesitarás uno nuevo.

Ingresa con tu correo y el código en el portal para definir tu contraseña y comenzar.

Saludos,
Equipo Heno Motita`,
		name,
		code,
		expiresText,
	)

	htmlBody := fmt.Sprintf(
		`<div style="font-family:Arial,sans-serif;max-width:600px;margin:auto;padding:24px;border:1px solid #e0e0e0;border-radius:8px;">
  <h2 style="color:#2e7d32;margin-top:0;">Heno Motita</h2>
  <p>Hola <strong>%s</strong>,</p>
  <p>Has sido registrado en el programa de monitoreo ambiental. Para activar tu cuenta usa el siguiente código:</p>
  <div style="background:#f5f5f5;border-radius:6px;padding:16px;text-align:center;font-size:24px;font-weight:bold;letter-spacing:4px;color:#2e7d32;">%s</div>
  <p>El código es válido hasta el <strong>%s</strong>.</p>
  <p>Ingresa con tu correo y el código en el portal para definir tu contraseña.</p>
  <p style="color:#757575;font-size:12px;">Si no esperabas este correo, puedes ignorarlo.</p>
</div>`,
		name,
		code,
		expiresText,
	)

	return service.send(
		ctx,
		to,
		subject,
		textBody,
		htmlBody,
	)
}

// send construye y envía un mensaje con SendGrid.
func (service *EmailService) send(
	ctx context.Context,
	to string,
	subject string,
	textBody string,
	htmlBody string,
) error {
	if !service.IsConfigured() {
		log.Printf(
			"[email][simulado] Para: %s | Asunto: %s\n%s",
			to,
			subject,
			textBody,
		)
		return nil
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	from := mail.NewEmail(
		service.config.FromName,
		service.config.From,
	)
	recipient := mail.NewEmail(
		"",
		to,
	)
	plainContent := mail.NewContent(
		"text/plain",
		textBody,
	)
	htmlContent := mail.NewContent(
		"text/html",
		htmlBody,
	)

	message := mail.NewV3MailInit(
		from,
		subject,
		recipient,
		plainContent,
		htmlContent,
	)

	response, err := service.client.Send(message)
	if err != nil {
		return fmt.Errorf(
			"no se pudo enviar el correo a %s: %w",
			to,
			err,
		)
	}

	if response.StatusCode >= 400 {
		return fmt.Errorf(
			"SendGrid rechazó el correo a %s: status %d - %s",
			to,
			response.StatusCode,
			strings.TrimSpace(response.Body),
		)
	}

	return nil
}