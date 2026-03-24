package game

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// LLAVE MAESTRA (Debe ser la misma en client.go para el WS)
var jwtKey = []byte("tu_secreto_super_glitch_2026")

type Claims struct {
	Username string `json:"username"`
	jwt.RegisteredClaims
}

// Generador de Tokens
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

// Handler de Login
func LoginHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "JSON inválido", http.StatusBadRequest)
			return
		}

		var dbPassword, avatarURL string
		query := `SELECT password_hash, avatar_url FROM users WHERE LOWER(username) = LOWER($1)`
		err := db.QueryRow(query, req.Username).Scan(&dbPassword, &avatarURL)

		if err != nil {
			fmt.Printf("⚠️ Usuario [%s] no encontrado\n", req.Username)
			http.Error(w, "Credenciales incorrectas", http.StatusUnauthorized)
			return
		}

		// Comparar Bcrypt
		err = bcrypt.CompareHashAndPassword([]byte(dbPassword), []byte(req.Password))
		if err != nil {
			fmt.Printf("🚫 Password error para [%s]\n", req.Username)
			http.Error(w, "Credenciales incorrectas", http.StatusUnauthorized)
			return
		}

		token, _ := GenerateToken(req.Username)

		fmt.Printf("✅ [%s] ha entrado al sistema\n", req.Username)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"token":      token,
			"username":   req.Username,
			"avatar_url": avatarURL,
		})
	}
}

// Handler de Registro (Para asegurar hashes perfectos)
func RegisterHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Username string `json:"username"`
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		json.NewDecoder(r.Body).Decode(&req)

		hash, _ := bcrypt.GenerateFromPassword([]byte(req.Password), 10)
		avatar := fmt.Sprintf("https://api.dicebear.com/7.x/pixel-art/svg?seed=%s", req.Username)

		query := `INSERT INTO users (username, email, password_hash, avatar_url) VALUES ($1, $2, $3, $4)`
		_, err := db.Exec(query, req.Username, req.Email, string(hash), avatar)

		if err != nil {
			http.Error(w, "Error al crear usuario", http.StatusConflict)
			return
		}

		w.WriteHeader(http.StatusCreated)
		fmt.Fprintf(w, "Usuario creado")
	}
}
