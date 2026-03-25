package game

import (
	"database/sql"
	"encoding/json"
	"log"
	"sync"
	"time"
)

// --- ESTRUCTURAS ---
type Question struct {
	ID           int    `json:"id"`
	QuestionText string `json:"question_text"`
	OptionA      string `json:"option_a"`
	OptionB      string `json:"option_b"`
	OptionC      string `json:"option_c"`
	OptionD      string `json:"option_d"`
}

type WSMessage struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

type AttackPayload struct {
	AttackerID string `json:"attacker_id"`
	TargetID   string `json:"target_id"`
	Type       string `json:"type"`
}

var Arsenal = map[string]struct {
	Cost   int
	Damage int
}{
	"Monstertify": {Cost: 1, Damage: 10},
	"Blur":        {Cost: 2, Damage: 25},
	"Blackout":    {Cost: 5, Damage: 60},
}

// --- EL HUB CON PERSISTENCIA ---
type Hub struct {
	Clients              map[*Client]bool
	Broadcast            chan []byte
	Register             chan *Client
	Unregister           chan *Client
	DB                   *sql.DB
	BroadcastAttack      chan AttackPayload
	CurrentCorrectOption string
	RoundWinners         []string // 👈 Guardamos IDs (strings), no punteros
	mu                   sync.Mutex
	GameState            string

	// 🔥 LA CLAVE: Mapas para que los puntos no se pierdan al reconectar
	PlayerTokens map[string]int
	PlayerHP     map[string]int
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
		PlayerTokens:    make(map[string]int),
		PlayerHP:        make(map[string]int),
	}
}

// --- LÓGICA DE JUEGO ---

func (h *Hub) SendRandomQuestion() {
	h.mu.Lock()
	if h.GameState != "esperando" {
		h.mu.Unlock()
		return
	}
	h.GameState = "trivia"
	h.RoundWinners = []string{} // Limpiamos ganadores
	h.mu.Unlock()

	var q Question
	var correctOption string
	query := `SELECT id, question_text, option_a, option_b, option_c, option_d, correct_option 
			  FROM questions ORDER BY RANDOM() LIMIT 1`

	err := h.DB.QueryRow(query).Scan(&q.ID, &q.QuestionText, &q.OptionA, &q.OptionB, &q.OptionC, &q.OptionD, &correctOption)
	if err != nil {
		h.mu.Lock()
		h.GameState = "esperando"
		h.mu.Unlock()
		return
	}

	h.mu.Lock()
	h.CurrentCorrectOption = correctOption
	h.mu.Unlock()

	msgPregunta, _ := json.Marshal(WSMessage{Type: "pregunta", Data: q})
	h.Broadcast <- msgPregunta

	h.syncAllClients("trivia")
	go h.startRoundTimer(q.ID)
}

func (h *Hub) startRoundTimer(questionID int) {
	time.Sleep(15 * time.Second)

	h.mu.Lock()
	h.GameState = "ataque"
	// Entregamos tokens basándonos en el ID de la lista de ganadores
	for i, playerID := range h.RoundWinners {
		if i == 0 { h.PlayerTokens[playerID] += 2 }
		if i == 1 { h.PlayerTokens[playerID] += 1 }
	}
	h.mu.Unlock()

	h.syncAllClients("ataque")
	go h.startAttackWindow()
}

func (h *Hub) startAttackWindow() {
	time.Sleep(10 * time.Second)
	h.mu.Lock()
	h.GameState = "esperando"
	h.mu.Unlock()
	h.syncAllClients("esperando")
}

// Sincroniza a todos los clientes con los datos reales de los mapas
func (h *Hub) syncAllClients(status string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for client := range h.Clients {
		msg, _ := json.Marshal(WSMessage{
			Type: "estado",
			Data: map[string]interface{}{
				"status": status,
				"tokens": h.PlayerTokens[client.ID],
				"hp":     h.PlayerHP[client.ID],
			},
		})
		client.Send <- msg
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.Register:
			h.mu.Lock()
			h.Clients[client] = true
			// Si es un jugador nuevo, le damos 100 HP
			if _, exists := h.PlayerHP[client.ID]; !exists {
				h.PlayerHP[client.ID] = 100
			}
			h.mu.Unlock()
			h.broadcastPlayersList()

		case client := <-h.Unregister:
			h.mu.Lock()
			if _, ok := h.Clients[client]; ok {
				delete(h.Clients, client)
				close(client.Send)
			}
			h.mu.Unlock()
			h.broadcastPlayersList()

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
			h.HandleAttack(attack.AttackerID, attack.TargetID, attack.Type)
		}
	}
}

func (h *Hub) HandleAttack(attackerID, targetID, attackName string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.GameState != "ataque" { return }

	info, exists := Arsenal[attackName]
	// ⚡ Chequeamos contra el MAPA, no contra el objeto de conexión
	if !exists || h.PlayerTokens[attackerID] < info.Cost {
		log.Printf("⚠️ %s no tiene fondos (Tiene: %d, Necesita: %d)", attackerID, h.PlayerTokens[attackerID], info.Cost)
		return
	}

	// Aplicamos daño en el MAPA
	if hp, exists := h.PlayerHP[targetID]; exists && hp > 0 {
		h.PlayerTokens[attackerID] -= info.Cost
		h.PlayerHP[targetID] -= info.Damage
		if h.PlayerHP[targetID] < 0 { h.PlayerHP[targetID] = 0 }

		// Notificamos a todos
		payload := map[string]interface{}{
			"attacker":        attackerID,
			"target":          targetID,
			"attack":          attackName,
			"new_hp":          h.PlayerHP[targetID],
			"attacker_tokens": h.PlayerTokens[attackerID],
		}
		msg, _ := json.Marshal(WSMessage{Type: "ataque_ejecutado", Data: payload})
		
		// Enviamos a todos
		h.mu.Unlock() // Soltamos un momento para no bloquear el broadcast
		h.Broadcast <- msg
		h.mu.Lock()

		if h.PlayerHP[targetID] <= 0 {
			log.Printf("💀 %s eliminado", targetID)
		}
	}
}

func (h *Hub) broadcastPlayersList() {
	h.mu.Lock()
	var players []string
	for client := range h.Clients {
		players = append(players, client.ID)
	}
	h.mu.Unlock()
	msg, _ := json.Marshal(WSMessage{Type: "lista_jugadores", Data: players})
	h.Broadcast <- msg
}