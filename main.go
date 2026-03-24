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

// Middleware para habilitar CORS en todas las rutas
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
	// 1. Conexión a la BD (Añadimos sslmode=require para Supabase)
	connStr := "postgresql://postgres.chpzuingsmjdocwhlqdr:E6HnkxAR8JrdkWsR@aws-0-us-west-2.pooler.supabase.com:5432/postgres?sslmode=require"

	db, err := sql.Open("postgres", connStr)
	http.HandleFunc("/api/login", game.LoginHandler(db))
	if err != nil {
		log.Fatalf("❌ Error configurando BD: %v", err)
	}
	defer db.Close()

	if err = db.Ping(); err != nil {
		log.Fatalf("❌ No se pudo conectar a la BD: %v", err)
	}
	fmt.Println("✅ Conectado a PostgreSQL exitosamente!")

	// 2. Inicializar el Hub y encenderlo en segundo plano
	hub := game.NewHub(db)
	go hub.Run()

	// 3. Definir Rutas en el Mux predeterminado
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		game.ServeWs(hub, w, r)
	})

	http.HandleFunc("/api/start-round", func(w http.ResponseWriter, r *http.Request) {
		hub.SendRandomQuestion()
		fmt.Fprintf(w, "Ronda iniciada. Pregunta enviada a los clientes.")
	})

	// 4. Configurar el puerto
	puerto := os.Getenv("PORT")
	if puerto == "" {
		puerto = "8080"
	}

	// 5. APLICAR EL MIDDLEWARE
	// En lugar de usar 'nil', pasamos nuestro Mux envuelto en la función enableCORS
	routerPrincipal := enableCORS(http.DefaultServeMux)

	fmt.Printf("🚀 Servidor corriendo en el puerto %s\n", puerto)
	fmt.Printf("🎯 API: http://localhost:%s/api/start-round\n", puerto)
	fmt.Printf("⚔️  WS: ws://localhost:%s/ws\n", puerto)

	// 6. INICIAR EL SERVIDOR (Esta línea debe ser la última)
	if err := http.ListenAndServe(":"+puerto, routerPrincipal); err != nil {
		log.Fatalf("❌ Error al iniciar el servidor: %v", err)
	}
}
