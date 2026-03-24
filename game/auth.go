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

var jwtKey = []byte("tu_secreto_super_glitch_2026")

type Claims struct {
	Username string `json:"username"`
	jwt.RegisteredClaims
}

type RegisterRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

func RegisterHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req RegisterRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Datos inválidos", http.StatusBadRequest)
			return
		}

		// 1. Cifrar la contraseña (Hash)
		// El "Cost" de 10 es el estándar de seguridad equilibrado
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), 10)
		if err != nil {
			http.Error(w, "Error al procesar contraseña", http.StatusInternalServerError)
			return
		}

		// 2. Generar un avatar automático con DiceBear (estilo Pixel Art)
		avatarURL := fmt.Sprintf("https://api.dicebear.com/7.x/pixel-art/svg?seed=%s", req.Username)

		// 3. Insertar en la Base de Datos
		query := `INSERT INTO users (username, email, password_hash, avatar_url) 
				  VALUES ($1, $2, $3, $4)`

		_, err = db.Exec(query, req.Username, req.Email, string(hashedPassword), avatarURL)

		if err != nil {
			// Si el error es por usuario duplicado
			fmt.Printf("❌ Error al registrar: %v\n", err)
			http.Error(w, "El usuario o email ya existe", http.StatusConflict)
			return
		}

		fmt.Printf("👤 Nuevo usuario registrado: %s\n", req.Username)

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{
			"message": "Usuario creado con éxito. ¡Bienvenido al Glitch!",
		})
	}
}

// 1. GENERADOR (Tu código está perfecto)
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

// 2. HANDLER DE LOGIN (Copia esto exactamente)
func LoginnHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "JSON inválido", http.StatusBadRequest)
			return
		}

		fmt.Printf("🔍 Buscando a: [%s]\n", req.Username)

		var dbPassword, avatarURL string
		// Usamos LOWER para evitar problemas de mayúsculas/minúsculas
		query := `SELECT password_hash, avatar_url FROM users WHERE LOWER(username) = LOWER($1)`
		err := db.QueryRow(query, req.Username).Scan(&dbPassword, &avatarURL)

		if err != nil {
			fmt.Printf("❌ Usuario [%s] no encontrado en DB\n", req.Username)
			http.Error(w, "Usuario no encontrado", http.StatusUnauthorized)
			return
		}

		// Comparar contraseña enviada con el hash de la DB
		err = bcrypt.CompareHashAndPassword([]byte(dbPassword), []byte(req.Password))
		if err != nil {
			fmt.Printf("🚫 Password incorrecto para [%s]\n", req.Username)
			http.Error(w, "Contraseña incorrecta", http.StatusUnauthorized)
			return
		}

		// Si llegamos aquí, todo es correcto
		token, _ := GenerateToken(req.Username)

		fmt.Printf("✅ Login exitoso: %s\n", req.Username)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"token":      token,
			"username":   req.Username,
			"avatar_url": avatarURL,
		})
	}
}
