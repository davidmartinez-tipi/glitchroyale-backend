package game

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"
)

// --- ESTRUCTURAS DE DATOS ---


type AttackPayload struct {
	AttackerID string `json:"attacker_id"`
	TargetID   string `json:"target_id"`
	Type       string `json:"type"`
}

// Arsenal define los costos y daños de los ataques disponibles
v
// --- EL HUB ---

type Hub struct {
	Clients              map[*Client]bool
	Broadcast            chan []byte
	Register             chan *Client
	Unregister           chan *Client
	DB                   *sql.DB
	BroadcastAttack      chan AttackPayload
	CurrentCorrectOption string
	RoundWinners         []*Client
	mu                   sync.Mutex
	GameState            string // "esperando", "trivia", "ataque"
}

func NewHub(db *sql.DB) *Hub {
	return &Hub{
		Broadcast:       make(chan []byte, 256),
		Register:        make(chan *Client),
		Unregister:      make(chan *Client),
		Clients:         make(map[*Client]bool),
		DB:              db,
		BroadcastAttack: make(chan AttackPayload, 256),
		GameState:       "esperando",
	}
}

// --- LÓGICA DE TRIVIA ---

func (h *Hub) SendRandomQuestion() {
	var q Question
	var correctOption string

	// 1. Obtener pregunta aleatoria de la base de datos
	query := `SELECT id, question_text, option_a, option_b, option_c, option_d, correct_option 
              FROM questions ORDER BY RANDOM() LIMIT 1`

	err := h.DB.QueryRow(query).Scan(&q.ID, &q.QuestionText, &q.OptionA, &q.OptionB, &q.OptionC, &q.OptionD, &correctOption)
	if err != nil {
		log.Println("❌ Error al obtener pregunta:", err)
		return
	}

	h.mu.Lock()
	h.CurrentCorrectOption = correctOption
	h.RoundWinners = make([]*Client, 0)
	h.GameState = "trivia"
	h.mu.Unlock()

	// 2. Notificar la pregunta
	msgPregunta, _ := json.Marshal(WSMessage{Type: "pregunta", Data: q})
	h.Broadcast <- msgPregunta

	// 3. Forzar cambio de estado en los clientes
	msgEstado, _ := json.Marshal(WSMessage{
		Type: "estado",
		Data: map[string]interface{}{
			"status": "trivia",
		},
	})
	h.Broadcast <- msgEstado

	log.Printf("📢 Pregunta [%d] enviada. Respuesta correcta: %s", q.ID, correctOption)

	// 4. Iniciar temporizador de la ronda
	go h.startRoundTimer(q.ID)
}

func (h *Hub) startRoundTimer(questionID int) {
	time.Sleep(15 * time.Second) // 15 segundos para responder

	h.mu.Lock()
	fmt.Printf("⏰ Tiempo agotado para pregunta %d. Ganadores: %d\n", questionID, len(h.RoundWinners))

	// Premiar a los más rápidos
	for i, winner := range h.RoundWinners {
		if i == 0 {
			winner.Tokens += 2
		} else if i == 1 {
			winner.Tokens += 1
		}
	}
	h.GameState = "ataque"
	h.mu.Unlock()

	// Notificar inicio de fase de ataque
	msg, _ := json.Marshal(WSMessage{
		Type: "estado",
		Data: map[string]interface{}{
			"status": "ataque",
		},
	})
	h.Broadcast <- msg

	go h.startAttackWindow()
}

func (h *Hub) startAttackWindow() {
	time.Sleep(10 * time.Second) // 10 segundos para atacar

	h.mu.Lock()
	h.GameState = "esperando"
	h.mu.Unlock()

	msg, _ := json.Marshal(WSMessage{
		Type: "estado",
		Data: map[string]interface{}{
			"status": "esperando",
		},
	})
	h.Broadcast <- msg
	log.Println("🛡️ Ronda terminada. Sistema en reposo.")
}

// --- LÓGICA DE PROCESAMIENTO ---

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.Register:
			h.Clients[client] = true
			log.Printf("👤 Jugador conectado: %s", client.ID)

		case client := <-h.Unregister:
			if _, ok := h.Clients[client]; ok {
				delete(h.Clients, client)
				close(client.Send)
			}

		case message := <-h.Broadcast:
			for client := range h.Clients {
				select {
				case client.Send <- message:
				default:
					close(client.Send)
					delete(h.Clients, client)
				}
			}

		case attack := <-h.BroadcastAttack:
			for client := range h.Clients {
				if client.ID == attack.AttackerID {
					h.HandleAttack(client, attack.TargetID, attack.Type)
					break
				}
			}
		}
	}
}

func (h *Hub) HandleAttack(attacker *Client, targetID string, attackName string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.GameState != "ataque" {
		log.Printf("🚫 %s intentó atacar fuera de tiempo", attacker.ID)
		return
	}

	info, exists := Arsenal[attackName]
	if !exists || attacker.Tokens < info.Cost {
		return
	}

	var target *Client
	for c := range h.Clients {
		if c.ID == targetID {
			target = c
			break
		}
	}

	if target != nil && target.HP > 0 {
		attacker.Tokens -= info.Cost
		target.HP -= info.Damage
		if target.HP < 0 {
			target.HP = 0
		}

		// Notificar a todos el evento visual
		payload := map[string]interface{}{
			"attacker": attacker.ID,
			"target":   target.ID,
			"attack":   attackName,
			"new_hp":   target.HP,
		}
		msg, _ := json.Marshal(WSMessage{Type: "ataque_ejecutado", Data: payload})
		h.Broadcast <- msg

		if target.HP <= 0 {
			h.EliminatePlayer(target)
		}
	}
}

func (h *Hub) EliminatePlayer(c *Client) {
	log.Printf("💀 %s eliminado", c.ID)
	msg, _ := json.Marshal(WSMessage{Type: "eliminacion", Data: c.ID})
	c.Send <- msg
}
