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

		var playerMsg PlayerMessage
		err = json.Unmarshal(msg, &playerMsg)
		if err != nil {
			log.Println("⚠️ Mensaje no válido:", string(msg))
			continue
		}

		// --- CASO 1: RESPUESTA A TRIVIA ---
		if playerMsg.Type == "respuesta" {
			c.Hub.mu.Lock()
			// Si la respuesta es correcta y no ha respondido ya
			if playerMsg.Data == c.Hub.CurrentCorrectOption {
				c.Hub.RoundWinners = append(c.Hub.RoundWinners, c)
				log.Printf("✅ %s acertó!", c.ID)
			}
			c.Hub.mu.Unlock()
		}

		// --- CASO 2: LANZAR UN ATAQUE ---
		if playerMsg.Type == "ataque" {
			// El data viene como un mapa/objeto: {target_id: "...", type: "..."}
			data, ok := playerMsg.Data.(map[string]interface{})
			if !ok {
				continue
			}

			targetID := data["target_id"].(string)
			tipoAtaque := data["type"].(string)

			// Validar si el jugador tiene tokens suficientes (ejemplo: costo 1)
			if c.Tokens >= 1 {
				c.Tokens -= 1 // Restamos el token
				log.Printf("⚔️ %s ataca a %s con %s", c.ID, targetID, tipoAtaque)

				// Enviamos el ataque al Hub para que lo procese y lo mande al rival
				c.Hub.BroadcastAttack <- AttackPayload{
					AttackerID: c.ID,
					TargetID:   targetID,
					Type:       tipoAtaque,
				}

				// 🔥 IMPORTANTE: Avisar al frontend que ahora tiene menos tokens
				c.sendPlayerState()
			}
		}
	}
}
func (c *Client) sendPlayerState() {
	stateMsg := map[string]interface{}{
		"type": "estado",
		"data": map[string]interface{}{
			"hp":     c.HP,
			"tokens": c.Tokens, // 👈 Esto habilitará los botones en React
			"status": "ataque", // O el estado actual del juego
		},
	}
	payload, _ := json.Marshal(stateMsg)
	c.Send <- payload
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
