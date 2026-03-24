package game

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

type Client struct {
	ID     string
	Hub    *Hub
	Conn   *websocket.Conn
	Send   chan []byte
	HP     int
	Tokens int
}

// writePump empuja los mensajes del servidor hacia el cliente (¡Esto faltaba!)
func (c *Client) writePump() {
	defer func() {
		c.Conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.Send:
			if !ok {
				// El Hub cerró el canal
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			if err := w.Close(); err != nil {
				return
			}
		}
	}
}

// readPump escucha los mensajes que vienen del cliente (Postman/React)
func (c *Client) readPump() {
	defer func() {
		c.Hub.Unregister <- c
		c.Conn.Close()
	}()
	for {
		_, msg, err := c.Conn.ReadMessage()
		if err != nil {
			break
		}

		// Intentamos convertir el JSON del jugador a nuestra estructura de Go
		var playerMsg PlayerMessage
		err = json.Unmarshal(msg, &playerMsg)
		if err != nil {
			log.Println("⚠️ Mensaje no válido del jugador:", string(msg))
			continue
		}

		// Si el jugador está enviando una respuesta a la trivia
		if playerMsg.Type == "respuesta" {
			c.Hub.mu.Lock() // Bloqueamos un microsegundo para evitar empates exactos

			// Comparamos lo que envió el jugador con la respuesta correcta actual
			if playerMsg.Data == c.Hub.CurrentCorrectOption {
				// Si es correcto, lo guardamos en la lista de ganadores en orden de llegada
				c.Hub.RoundWinners = append(c.Hub.RoundWinners, c)
				log.Printf("✅ ¡Respuesta CORRECTA del jugador!")
			} else {
				log.Printf("❌ Respuesta INCORRECTA")
			}

			c.Hub.mu.Unlock()
		}
	}
}

// ServeWs maneja la conexión inicial
func ServeWs(hub *Hub, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Error al conectar WebSocket:", err)
		return
	}
	client := &Client{
		ID:     fmt.Sprintf("Player_%d", time.Now().UnixNano()%1000),
		Hub:    hub,
		Conn:   conn,
		Send:   make(chan []byte, 256),
		HP:     100, // [cite: 11]
		Tokens: 0,
	}

	// Iniciamos las dos tareas en paralelo
	go client.writePump() // Enviar al cliente
	go client.readPump()  // Escuchar al cliente
}
