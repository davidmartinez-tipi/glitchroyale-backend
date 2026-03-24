package game

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var jwtKey = []byte("tu_secreto_super_glitch_2026") // Usa una variable de entorno en producción

type Claims struct {
	Username string `json:"username"`
	jwt.RegisteredClaims
}

// Generar un token que dura 24 horas
func GenerateToken(username string) (string, error) {
	expirationTime := time.Now().Add(24 * time.Hour)
	claims := &Claims{
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtKey)
}
