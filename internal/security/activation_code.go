package security

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	activationAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	activationCodeSize = 8
	activationCodeLife = 7 * 24 * time.Hour
)

// GenerateActivationCode genera un código como:
//
// # HM-ABCD-2345
//
// Se excluyen caracteres visualmente ambiguos como O, 0, I y 1.
func GenerateActivationCode() (string, error) {
	characters := make(
		[]byte,
		activationCodeSize,
	)

	alphabetLength := big.NewInt(
		int64(len(activationAlphabet)),
	)

	for index := range characters {
		randomIndex, err := rand.Int(
			rand.Reader,
			alphabetLength,
		)
		if err != nil {
			return "", fmt.Errorf(
				"no se pudo generar el código de activación: %w",
				err,
			)
		}

		characters[index] = activationAlphabet[randomIndex.Int64()]
	}

	rawCode := string(characters)

	return fmt.Sprintf(
		"HM-%s-%s",
		rawCode[:4],
		rawCode[4:],
	), nil
}

// HashActivationCode cifra el código antes de almacenarlo.
func HashActivationCode(
	code string,
) (string, error) {
	normalizedCode := NormalizeActivationCode(code)

	if normalizedCode == "" {
		return "", fmt.Errorf(
			"el código de activación está vacío",
		)
	}

	hash, err := bcrypt.GenerateFromPassword(
		[]byte(normalizedCode),
		bcrypt.DefaultCost,
	)
	if err != nil {
		return "", fmt.Errorf(
			"no se pudo proteger el código de activación: %w",
			err,
		)
	}

	return string(hash), nil
}

// CheckActivationCode compara el código recibido
// contra el hash almacenado.
func CheckActivationCode(
	hash string,
	code string,
) bool {
	hash = strings.TrimSpace(hash)

	if hash == "" {
		return false
	}

	normalizedCode := NormalizeActivationCode(code)

	if normalizedCode == "" {
		return false
	}

	return bcrypt.CompareHashAndPassword(
		[]byte(hash),
		[]byte(normalizedCode),
	) == nil
}

// NormalizeActivationCode normaliza códigos escritos
// con espacios o letras minúsculas.
func NormalizeActivationCode(
	code string,
) string {
	return strings.ToUpper(
		strings.ReplaceAll(
			strings.TrimSpace(code),
			" ",
			"",
		),
	)
}

// CalculateActivationCodeExpiration limita la vigencia
// del código a siete días o a la fecha final de la cuadrilla.
func CalculateActivationCodeExpiration(
	now time.Time,
	crewEndAt time.Time,
) time.Time {
	expiration := now.Add(
		activationCodeLife,
	)

	if crewEndAt.Before(expiration) {
		return crewEndAt
	}

	return expiration
}
