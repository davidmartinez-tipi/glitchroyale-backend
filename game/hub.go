package game

import (
	"database/sql"
	"encoding/json"
	"log"
	"sync"
	"time"
)

// --- 1. ESTRUCTURAS DE DATOS ---
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

// --- 2. EL HUB ---
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
	GameState            string
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

// --- 3. EL RADAR (MULTIJUGADOR) ---
func (h *Hub) broadcastPlayersList() {
	var players []string
	for client := range h.Clients {
		players = append(players, client.ID)
	}
	msg, _ := json.Marshal(WSMessage{Type: "lista_jugadores", Data: players})

	// 🔥 Enviamos la lista al túnel principal para no bloquear el servidor
	h.Broadcast <- msg
}

// --- 4. LÓGICA DE TRIVIA ---
func (h *Hub) SendRandomQuestion() {
	h.mu.Lock()
	// 🚨 EL CANDADO: Si no estamos esperando, ignoramos el clic
	if h.GameState != "esperando" {
		h.mu.Unlock()
		log.Println("⚠️ Clic doble ignorado: Ya hay una ronda en curso.")
		return
	}
	h.GameState = "preparando" // Bloqueamos instantáneamente
	h.mu.Unlock()

	var q Question
	var correctOption string
	query := `SELECT id, question_text, option_a, option_b, option_c, option_d, correct_option 
			  FROM questions ORDER BY RANDOM() LIMIT 1`

	err := h.DB.QueryRow(query).Scan(&q.ID, &q.QuestionText, &q.OptionA, &q.OptionB, &q.OptionC, &q.OptionD, &correctOption)
	if err != nil {
		h.mu.Lock()
		h.GameState = "esperando" // Quitamos el candado si hay error de DB
		h.mu.Unlock()
		log.Println("❌ Error al obtener pregunta:", err)
		return
	}

	h.mu.Lock()
	h.CurrentCorrectOption = correctOption
	h.RoundWinners = make([]*Client, 0)
	h.GameState = "trivia"
	h.mu.Unlock()

	msgPregunta, _ := json.Marshal(WSMessage{Type: "pregunta", Data: q})
	h.Broadcast <- msgPregunta

	msgEstado, _ := json.Marshal(WSMessage{
		Type: "estado",
		Data: map[string]interface{}{"status": "trivia"},
	})
	h.Broadcast <- msgEstado

	log.Printf("📢 Pregunta [%d] enviada al túnel", q.ID)
	go h.startRoundTimer(q.ID)
}

func (h *Hub) startRoundTimer(questionID int) {
	time.Sleep(15 * time.Second) // El tiempo que dura la pregunta

	h.mu.Lock()
	// 1. Repartimos los tokens a los ganadores
	for i, winner := range h.RoundWinners {
		if i == 0 {
			winner.Tokens += 2
			log.Printf("🏆 2 tokens para %s", winner.ID)
		} else if i == 1 {
			winner.Tokens += 1
		}
	}
	h.GameState = "ataque"
	h.mu.Unlock()

	// 2. 🔥 LA MAGIA: Le enviamos a CADA jugador su saldo exacto y actual
	for client := range h.Clients {
		msg, _ := json.Marshal(WSMessage{
			Type: "estado",
			Data: map[string]interface{}{
				"status": "ataque",
				"tokens": client.Tokens, // ¡Aquí viaja el dinero a React!
				"hp":     client.HP,
			},
		})
		// Enviamos el mensaje personal al canal del cliente
		client.Send <- msg
	}

	go h.startAttackWindow()
}
func (h *Hub) startAttackWindow() {
	time.Sleep(10 * time.Second) // El tiempo que dura la fase de ataque

	h.mu.Lock()
	h.GameState = "esperando"
	h.mu.Unlock()

	// Actualizamos a todos para que vuelvan a la pantalla de espera con su HP final
	for client := range h.Clients {
		msg, _ := json.Marshal(WSMessage{
			Type: "estado",
			Data: map[string]interface{}{
				"status": "esperando",
				"tokens": client.Tokens,
				"hp":     client.HP,
			},
		})
		client.Send <- msg
	}
}

// --- 5. BUCLE PRINCIPAL (EL REPARTIDOR DE MENSAJES) ---
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.Register:
			h.Clients[client] = true
			log.Printf("👤 Jugador conectado: %s", client.ID)
			h.broadcastPlayersList()

		case client := <-h.Unregister:
			if _, ok := h.Clients[client]; ok {
				delete(h.Clients, client)
				close(client.Send)
				log.Printf("🔌 Jugador desconectado: %s", client.ID)
				h.broadcastPlayersList()
			}

		// 🔥 ESTA ES LA PIEZA CRÍTICA QUE FALTABA 🔥
		// Sin esto, los mensajes se quedan en el servidor y nunca viajan a React
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

// --- 6. COMBATE ---
func (h *Hub) HandleAttack(attacker *Client, targetID string, attackName string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.GameState != "ataque" {
		log.Printf("🚫 %s intentó atacar fuera de tiempo", attacker.ID)
		return
	}

	info, exists := Arsenal[attackName]
	if !exists || attacker.Tokens < info.Cost {
		log.Printf("⚠️ %s no tiene fondos para %s", attacker.ID, attackName)
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

		// 🔥 Aquí enviamos los tokens actualizados al atacante y el HP a la víctima
		payload := map[string]interface{}{
			"attacker":        attacker.ID,
			"target":          target.ID,
			"attack":          attackName,
			"new_hp":          target.HP,
			"attacker_tokens": attacker.Tokens,
		}

		// Mensaje directo sin pasar por intermediarios
		msg, _ := json.Marshal(WSMessage{Type: "ataque_ejecutado", Data: payload})
		for c := range h.Clients {
			c.Send <- msg
		}

		log.Printf("💥 %s hizo %d de daño a %s. HP de víctima: %d", attacker.ID, info.Damage, target.ID, target.HP)

		if target.HP <= 0 {
			h.EliminatePlayer(target)
		}
	}
}

func (h *Hub) EliminatePlayer(c *Client) {
	msg, _ := json.Marshal(WSMessage{Type: "eliminacion", Data: c.ID})
	c.Send <- msg
	h.broadcastPlayersList() // Actualizamos lista porque alguien murió
}
