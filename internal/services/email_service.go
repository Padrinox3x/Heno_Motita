package services

import (
	"context"
	"fmt"
	"log"
	"net/smtp"
	"strings"
	"time"
)

// SMTPConfig concentra los datos necesarios
// para enviar correos por SMTP.
type SMTPConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	From     string
	FromName string
}

// EmailService gestiona el envío de correos
// de activación por SMTP (p. ej. Gmail).
type EmailService struct {
	config   SMTPConfig
	hostPort string
	auth     smtp.Auth
}

// NewEmailService crea el servicio de correo.
//
// Si no se configura SMTP_HOST, el servicio no envía
// correos reales: imprime en consola el contenido
// que habría enviado (útil en desarrollo).
func NewEmailService(
	config SMTPConfig,
) *EmailService {
	config.From = strings.TrimSpace(config.From)
	config.FromName = strings.TrimSpace(config.FromName)

	if config.FromName == "" {
		config.FromName = "Heno Motita"
	}

	port := strings.TrimSpace(config.Port)
	if port == "" {
		port = "587"
	}

	service := &EmailService{
		config:   config,
		hostPort: netJoinHostPort(
			strings.TrimSpace(config.Host),
			port,
		),
	}

	if config.Host != "" &&
		config.User != "" &&
		config.Password != "" {
		service.auth = smtp.PlainAuth(
			"",
			config.User,
			config.Password,
			strings.TrimSpace(config.Host),
		)
	}

	return service
}

// IsConfigured indica si hay un servidor SMTP
// real configurado.
func (service *EmailService) IsConfigured() bool {
	return service.auth != nil &&
		service.hostPort != "" &&
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

// send construye y envía un mensaje de correo.
func (service *EmailService) send(
	ctx context.Context,
	to string,
	subject string,
	textBody string,
	htmlBody string,
) error {
	from := service.config.From

	if service.config.FromName != "" {
		from = fmt.Sprintf(
			"%s <%s>",
			service.config.FromName,
			service.config.From,
		)
	}

	message := buildRFC822Message(
		from,
		to,
		subject,
		textBody,
		htmlBody,
	)

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

	if err := smtp.SendMail(
		service.hostPort,
		service.auth,
		service.config.From,
		[]string{to},
		[]byte(message),
	); err != nil {
		return fmt.Errorf(
			"no se pudo enviar el correo a %s: %w",
			to,
			err,
		)
	}

	return nil
}

// buildRFC822Message arma un correo multipart con
// versión en texto plano y en HTML.
func buildRFC822Message(
	from string,
	to string,
	subject string,
	textBody string,
	htmlBody string,
) string {
	boundary := "heno-motita-boundary"

	headers := []string{
		"From: " + from,
		"To: " + to,
		"Subject: " + subject,
		"MIME-Version: 1.0",
		"Content-Type: multipart/alternative; boundary=" + boundary,
		"",
	}

	textBody = strings.ReplaceAll(
		textBody,
		"\n",
		"\r\n",
	)

	htmlBody = strings.ReplaceAll(
		htmlBody,
		"\n",
		"\r\n",
	)

	parts := []string{
		"--" + boundary,
		"Content-Type: text/plain; charset=UTF-8",
		"",
		textBody,
		"--" + boundary,
		"Content-Type: text/html; charset=UTF-8",
		"",
		htmlBody,
		"--" + boundary + "--",
		"",
	}

	return strings.Join(
		append(headers, parts...),
		"\r\n",
	)
}

// netJoinHostPort une host y puerto con tolerancia
// a valores ya formados como "host:587".
func netJoinHostPort(
	host string,
	port string,
) string {
	host = strings.TrimSpace(host)
	port = strings.TrimSpace(port)

	if host == "" {
		return ""
	}

	if strings.Contains(host, ":") {
		return host
	}

	if port == "" {
		return host
	}

	return fmt.Sprintf(
		"%s:%s",
		host,
		port,
	)
}
