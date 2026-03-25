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

// PlayerMessage define la estructura de lo que recibimos de React
type PlayerMessage struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

type Client struct {
	ID   string
	Hub  *Hub
	Conn *websocket.Conn
	Send chan []byte
}

func (c *Client) writePump() {
	defer func() {
		c.Conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.Send:
			if !ok {
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
			// ✅ CORRECCIÓN: Guardamos el ID (string), no el cliente completo
			if playerMsg.Data == c.Hub.CurrentCorrectOption {
				c.Hub.RoundWinners = append(c.Hub.RoundWinners, c.ID)
				log.Printf("✅ %s acertó!", c.ID)
			}
			c.Hub.mu.Unlock()
		}

		// --- CASO 2: LANZAR UN ATAQUE ---
		if playerMsg.Type == "ataque" {
			data, ok := playerMsg.Data.(map[string]interface{})
			if !ok {
				continue
			}

			targetID, _ := data["target_id"].(string)
			tipoAtaque, _ := data["type"].(string)

			// ✅ CORRECCIÓN: No restamos tokens aquí.
			// Simplemente enviamos la petición al Hub.
			// El Hub decidirá si tienes dinero y ejecutará el ataque.
			log.Printf("⚔️ %s quiere atacar a %s con %s", c.ID, targetID, tipoAtaque)

			c.Hub.BroadcastAttack <- AttackPayload{
				AttackerID: c.ID,
				TargetID:   targetID,
				Type:       tipoAtaque,
			}
		}
	}
}

// ✅ CORRECCIÓN: Ahora consulta los mapas del Hub para dar datos reales
func (c *Client) sendPlayerState() {
	c.Hub.mu.Lock()
	hp := c.Hub.PlayerHP[c.ID]
	tokens := c.Hub.PlayerTokens[c.ID]
	status := c.Hub.GameState
	c.Hub.mu.Unlock()

	stateMsg := map[string]interface{}{
		"type": "estado",
		"data": map[string]interface{}{
			"hp":     hp,
			"tokens": tokens,
			"status": status,
		},
	}
	payload, _ := json.Marshal(stateMsg)
	c.Send <- payload
}

func ServeWs(hub *Hub, w http.ResponseWriter, r *http.Request) {
	tokenString := r.URL.Query().Get("token")

	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
		return jwtKey, nil
	})

	if err != nil || !token.Valid {
		log.Printf("🚫 Error validando JWT: %v", err)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	conn, _ := upgrader.Upgrade(w, r, nil)

	client := &Client{
		ID:   claims.Username,
		Hub:  hub,
		Conn: conn,
		Send: make(chan []byte, 256),
	}

	hub.Register <- client
	go client.writePump()
	go client.readPump()

	// Le enviamos su estado inicial nada más conectar
	client.sendPlayerState()
}
