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
	Clients    map[*Client]bool
	Broadcast  chan []byte
	Register   chan *Client
	Unregister chan *Client
	DB         *sql.DB

	CurrentCorrectOption string
	RoundWinners         []*Client
	mu                   sync.Mutex
}

func NewHub(db *sql.DB) *Hub {
	return &Hub{
		Broadcast:  make(chan []byte),
		Register:   make(chan *Client),
		Unregister: make(chan *Client),
		Clients:    make(map[*Client]bool),
		DB:         db,
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
			fmt.Println("🎮 Nuevo jugador conectado a la sala de batalla.")
		case client := <-h.Unregister:
			if _, ok := h.Clients[client]; ok {
				delete(h.Clients, client)
				close(client.Send)
				fmt.Println("❌ Jugador desconectado.")
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
		}
	}
}
