package game

import (
	"database/sql"
	"encoding/json"
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
			http.Error(w, "JSON inválido", http.StatusBadRequest)
			return
		}

		// 1. Buscar usuario en la DB
		var dbPassword, avatarURL string
		query := `SELECT password_hash, avatar_url FROM users WHERE username = $1`
		err := db.QueryRow(query, req.Username).Scan(&dbPassword, &avatarURL)

		if err == sql.ErrNoRows {
			http.Error(w, "Usuario no encontrado", http.StatusUnauthorized)
			return
		}

		// 2. Comparar contraseñas (Bcrypt)
		err = bcrypt.CompareHashAndPassword([]byte(dbPassword), []byte(req.Password))
		if err != nil {
			http.Error(w, "Contraseña incorrecta", http.StatusUnauthorized)
			return
		}

		// 3. Generar el JWT (Usando la función que creamos antes)
		token, err := GenerateToken(req.Username)
		if err != nil {
			http.Error(w, "Error al crear token", http.StatusInternalServerError)
			return
		}

		// 4. Responder con el éxito
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(LoginResponse{
			Token:     token,
			Username:  req.Username,
			AvatarURL: avatarURL,
		})
	}
}
