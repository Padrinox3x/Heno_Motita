package security

import (
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

const minimumPasswordLength = 8

// HashPassword cifra una contraseña antes de almacenarla.
func HashPassword(password string) (string, error) {
	password = strings.TrimSpace(password)

	if len(password) < minimumPasswordLength {
		return "", fmt.Errorf(
			"la contraseña debe tener al menos %d caracteres",
			minimumPasswordLength,
		)
	}

	if len([]byte(password)) > 72 {
		return "", fmt.Errorf(
			"la contraseña no puede superar los 72 bytes",
		)
	}

	hash, err := bcrypt.GenerateFromPassword(
		[]byte(password),
		bcrypt.DefaultCost,
	)
	if err != nil {
		return "", fmt.Errorf(
			"no se pudo cifrar la contraseña: %w",
			err,
		)
	}

	return string(hash), nil
}

// CheckPassword compara la contraseña ingresada con el hash almacenado.
func CheckPassword(passwordHash string, password string) error {
	if err := bcrypt.CompareHashAndPassword(
		[]byte(passwordHash),
		[]byte(password),
	); err != nil {
		return fmt.Errorf("contraseña incorrecta")
	}

	return nil
}
