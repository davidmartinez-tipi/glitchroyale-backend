package game

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/golang-jwt/jwt/v5"
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
	// 1. Extraer el token de la URL: ws://.../ws?token=XXXX
	tokenString := r.URL.Query().Get("token")

	// 2. Validar el Token
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
		return jwtKey, nil
	})

	if err != nil || !token.Valid {
		log.Println("🚫 Intento de conexión sin token válido")
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	// 3. Si es válido, conectar con su nombre real
	conn, _ := upgrader.Upgrade(w, r, nil)
	client := &Client{
		ID:     claims.Username, // 👈 Ahora el ID es su nombre de usuario real
		Hub:    hub,
		Conn:   conn,
		Send:   make(chan []byte, 256),
		HP:     100,
		Tokens: 0,
	}
	hub.Register <- client
	go client.writePump()
	go client.readPump()
}
