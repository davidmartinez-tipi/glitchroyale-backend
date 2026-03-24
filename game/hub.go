package game

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"
)

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
}

func NewHub(db *sql.DB) *Hub {
	return &Hub{
		Broadcast:       make(chan []byte),
		Register:        make(chan *Client),
		Unregister:      make(chan *Client),
		Clients:         make(map[*Client]bool),
		DB:              db,
		BroadcastAttack: make(chan AttackPayload),
	}
}

func (h *Hub) SendRandomQuestion() {
	var q Question
	var correctOption string

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
	h.mu.Unlock()

	msg := WSMessage{Type: "pregunta", Data: q}
	jsonMsg, _ := json.Marshal(msg)
	h.Broadcast <- jsonMsg

	fmt.Println("📢 Pregunta enviada. La respuesta correcta es oculta:", correctOption)

	// 🔥 SOLUCIÓN: Le pasamos el ID de la pregunta al reloj
	go h.startRoundTimer(q.ID)
}

// 🔥 SOLUCIÓN: La función ahora acepta el questionID (int)
func (h *Hub) startRoundTimer(questionID int) {
	time.Sleep(20 * time.Second)

	h.mu.Lock()
	fmt.Printf("⏰ ¡Tiempo para la pregunta %d! Hubo %d respuestas correctas.\n", questionID, len(h.RoundWinners))

	for i, winner := range h.RoundWinners {
		if i == 0 {
			winner.Tokens += 2
			fmt.Println("🏆 1er Lugar! Se le otorgan 2 tokens. Total:", winner.Tokens)
		} else if i == 1 {
			winner.Tokens += 1
			fmt.Println("🥈 2do Lugar! Se le otorga 1 token. Total:", winner.Tokens)
		}
	}
	h.mu.Unlock()

	msg := WSMessage{
		Type: "inicio_ataque",
		Data: "¡Ataquen! Tienen 8 segundos para usar sus tokens.",
	}
	jsonMsg, _ := json.Marshal(msg)
	h.Broadcast <- jsonMsg

	go h.startAttackWindow()
}

func (h *Hub) startAttackWindow() {
	fmt.Println("⚔️ Ventana de ataque abierta (8 segundos)...")
	time.Sleep(8 * time.Second)

	fmt.Println("🛡️ Ventana de ataque CERRADA.")
	msg := WSMessage{
		Type: "fin_ronda",
		Data: "La ronda ha terminado. Preparando siguiente pregunta...",
	}
	jsonMsg, _ := json.Marshal(msg)
	h.Broadcast <- jsonMsg
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.Register:
			h.Clients[client] = true
		case client := <-h.Unregister:
			if _, ok := h.Clients[client]; ok {
				delete(h.Clients, client)
				close(client.Send)
			}

		// 🔥 Lógica para procesar el ATAQUE
		case attack := <-h.BroadcastAttack:
			for client := range h.Clients {
				// 1. Buscamos al objetivo (Target)
				if client.ID == attack.TargetID {
					// 2. Aplicamos el daño/efecto
					client.HP -= 10 // Ejemplo: daño base

					// 3. Le avisamos al objetivo que fue atacado
					msg := map[string]interface{}{
						"type": "ataque_ejecutado",
						"data": map[string]interface{}{
							"attacker": attack.AttackerID,
							"attack":   attack.Type,
							"new_hp":   client.HP,
						},
					}
					payload, _ := json.Marshal(msg)
					client.Send <- payload

					log.Printf("💥 %s recibió un %s de %s. HP restante: %d",
						client.ID, attack.Type, attack.AttackerID, client.HP)
				}
			}
		}
	}
}
func (h *Hub) HandleAttack(attacker *Client, targetID string, attackName string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// 1. Validar si el ataque existe en nuestro arsenal
	info, exists := Arsenal[attackName]
	if !exists {
		return
	}

	// 2. Validar si el atacante tiene tokens suficientes [cite: 19]
	if attacker.Tokens < info.Cost {
		log.Printf("⚠️ %s no tiene tokens suficientes", attacker.ID)
		return
	}

	// 3. Buscar al rival (target) en la sala
	var target *Client
	for c := range h.Clients {
		if c.ID == targetID {
			target = c
			break
		}
	}

	// 4. Aplicar daño si el rival existe y tiene vida
	if target != nil && target.HP > 0 {
		attacker.Tokens -= info.Cost
		target.HP -= info.Damage
		log.Printf("⚔️ %s atacó a %s con %s. HP Rival: %d", attacker.ID, target.ID, attackName, target.HP)

		// 5. Notificar a TODOS el ataque para el efecto visual [cite: 32]
		payload := map[string]interface{}{
			"attacker": attacker.ID,
			"target":   target.ID,
			"attack":   attackName,
			"new_hp":   target.HP,
		}
		msg, _ := json.Marshal(WSMessage{Type: "ataque_ejecutado", Data: payload})
		h.Broadcast <- msg

		// 6. Verificar eliminación [cite: 10]
		if target.HP <= 0 {
			target.HP = 0
			h.EliminatePlayer(target)
		}
	}
}

func (h *Hub) EliminatePlayer(c *Client) {
	log.Printf("💀 %s ha sido eliminado", c.ID)
	msg, _ := json.Marshal(WSMessage{Type: "eliminacion", Data: c.ID})
	c.Send <- msg // Mensaje de Game Over personal
}

type AttackPayload struct {
	AttackerID string `json:"attacker_id"`
	TargetID   string `json:"target_id"`
	Type       string `json:"type"`
}
