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

func main() {
	// 1. Conexión a la BD
	connStr := "postgresql://postgres.chpzuingsmjdocwhlqdr:E6HnkxAR8JrdkWsR@aws-0-us-west-2.pooler.supabase.com:5432/postgres"

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("❌ Error configurando BD: %v", err)
	}
	defer db.Close()

	if err = db.Ping(); err != nil {
		log.Fatalf("❌ No se pudo conectar a la BD: %v", err)
	}
	fmt.Println("✅ Conectado a PostgreSQL exitosamente!")

	// 2. Inicializar el Hub pasándole la BD
	hub := game.NewHub(db)
	go hub.Run()

	// 3. Rutas
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		game.ServeWs(hub, w, r)
	})

	// RUTA DE PRUEBA: Al visitar esta URL, el servidor enviará la pregunta a los WebSockets
	http.HandleFunc("/api/start-round", func(w http.ResponseWriter, r *http.Request) {
		// 🔥 ESTAS 3 LÍNEAS ARREGLAN EL ERROR DE LA IMAGEN:
		w.Header().Set("Access-Control-Allow-Origin", "*") // Permite que cualquier sitio lo llame
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		// Si es una petición de tipo OPTIONS (pre-vuelo), respondemos OK
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		hub.SendRandomQuestion()
		fmt.Fprintf(w, "Ronda iniciada. Pregunta enviada a los clientes.")
	})

	puerto := os.Getenv("PORT")
	if puerto == "" {
		puerto = "8080" // Si estamos en local, usamos 8080
	}

	fmt.Printf("🚀 Servidor corriendo en el puerto %s\n", puerto)

	// Asegúrate de agregar los dos puntos ":" antes de la variable
	if err := http.ListenAndServe(":"+puerto, nil); err != nil {
		log.Fatalf("❌ Error: %v", err)
	}
	fmt.Printf("🚀 Servidor corriendo en http://localhost%s\n", puerto)
	fmt.Println("⚔️  Sala de batalla lista esperando jugadores en ws://localhost:8080/ws")
	fmt.Println("🎯 Para lanzar una pregunta, visita: http://localhost:8080/api/start-round")

	if err := http.ListenAndServe(puerto, nil); err != nil {
		log.Fatalf("❌ Error: %v", err)
	}
}
