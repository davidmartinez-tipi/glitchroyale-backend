package game

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"

	"golang.org/x/crypto/bcrypt"
)

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token     string `json:"token"`
	Username  string `json:"username"`
	AvatarURL string `json:"avatar_url"`
}

func LoginHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req LoginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			fmt.Println("❌ Error decodificando JSON:", err)
			http.Error(w, "JSON inválido", http.StatusBadRequest)
			return
		}

		fmt.Printf("🔍 Intentando login para: [%s]\n", req.Username)

		// 1. Buscar usuario (Usamos LOWER para evitar líos de mayúsculas)
		var dbPassword, avatarURL string
		query := `SELECT password_hash, avatar_url FROM users WHERE LOWER(username) = LOWER($1)`
		err := db.QueryRow(query, req.Username).Scan(&dbPassword, &avatarURL)

		if err != nil {
			if err == sql.ErrNoRows {
				fmt.Printf("⚠️ Usuario [%s] no encontrado en la base de datos\n", req.Username)
			} else {
				fmt.Printf("❌ Error de base de datos: %v\n", err)
			}
			http.Error(w, "Usuario no encontrado", http.StatusUnauthorized)
			return
		}

		fmt.Printf("🔐 Usuario encontrado. Comparando hash...\n")

		// 2. Comparar contraseñas
		err = bcrypt.CompareHashAndPassword([]byte(dbPassword), []byte(req.Password))
		if err != nil {
			fmt.Printf("🚫 Contraseña incorrecta para [%s]. Error: %v\n", req.Username, err)
			http.Error(w, "Contraseña incorrecta", http.StatusUnauthorized)
			return
		}

		// 3. Generar el JWT
		token, err := GenerateToken(req.Username)
		if err != nil {
			fmt.Println("❌ Error generando JWT:", err)
			http.Error(w, "Error al crear token", http.StatusInternalServerError)
			return
		}

		fmt.Printf("✅ Login exitoso para [%s]!\n", req.Username)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(LoginResponse{
			Token:     token,
			Username:  req.Username,
			AvatarURL: avatarURL,
		})
	}
}
