package main

import (
	"database/sql"
	"fmt"
	"glitchroyale/game"
	"log"
	"net/http"
	"os"

	_ "github.com/lib/pq"
)

// Middleware para habilitar CORS (Crucial para que React y Render hablen)
func enableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func main() {
	// 1. Conexión a la BD con SSL para Supabase
	connStr := "postgresql://postgres.chpzuingsmjdocwhlqdr:E6HnkxAR8JrdkWsR@aws-0-us-west-2.pooler.supabase.com:5432/postgres?sslmode=require"
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("❌ Error configurando BD: %v", err)
	}
	defer db.Close()

	if err = db.Ping(); err != nil {
		log.Fatalf("❌ No se pudo conectar a la BD: %v", err)
	}
	fmt.Println("✅ Conectado a PostgreSQL exitosamente!")

	// 2. Inicializar el Hub y el motor de juego
	hub := game.NewHub(db)
	go hub.Run()

	// 3. Definir Rutas de la API
	mux := http.NewServeMux() // Usamos un Mux limpio

	mux.HandleFunc("/api/login", game.LoginHandler(db))
	mux.HandleFunc("/api/register", game.RegisterHandler(db))

	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		game.ServeWs(hub, w, r)
	})

	// 🔥 MEJORA CRÍTICA: Inicio de ronda NO BLOQUEANTE
	mux.HandleFunc("/api/start-round", func(w http.ResponseWriter, r *http.Request) {
		// Lanzamos la lógica en una goroutine para que el HTTP responda 200 OK rápido
		go func() {
			log.Println("📢 Disparando nueva pregunta a los clientes...")
			hub.SendRandomQuestion()
		}()

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status": "success", "message": "Ronda disparada"}`)
	})

	// 4. Configurar el puerto para Render
	puerto := os.Getenv("PORT")
	if puerto == "" {
		puerto = "8080"
	}

	// 5. Aplicar Middleware y Encender
	routerConCORS := enableCORS(mux)

	fmt.Printf("🚀 GlitchRoyale Engine en puerto %s\n", puerto)

	if err := http.ListenAndServe(":"+puerto, routerConCORS); err != nil {
		log.Fatalf("❌ Error fatal: %v", err)
	}
}
