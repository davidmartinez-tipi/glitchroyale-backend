package game

// Question representa los datos que enviaremos al frontend.
// NOTA: Omitimos deliberadamente "correct_option" para evitar trampas.
type Question struct {
	ID           int    `json:"id"`
	QuestionText string `json:"question_text"`
	OptionA      string `json:"option_a"`
	OptionB      string `json:"option_b"`
	OptionC      string `json:"option_c"`
	OptionD      string `json:"option_d"`
}
type AttackInfo struct {
	Name     string
	Cost     int
	Damage   int
	Category string
}

// Mapa global con los datos oficiales del documento de diseño
var Arsenal = map[string]AttackInfo{
	"Monstertify": {Name: "Monstertify", Cost: 1, Damage: 8, Category: "Rostro"}, //
	"Blur":        {Name: "Blur", Cost: 2, Damage: 10, Category: "Interfaz"},     // [cite: 38]
	"Blackout":    {Name: "Blackout", Cost: 5, Damage: 30, Category: "Impacto"},  // [cite: 41]
	// Puedes agregar el resto (Clownify, Terremoto, etc.) siguiendo el mismo patrón
}

type AttackRequest struct {
	TargetID string `json:"target_id"` // A quién atacamos
	Type     string `json:"type"`      // Qué ataque usamos (ej. "Monstertify")
}

// WSMessage es la estructura estándar para enviar mensajes por el WebSocket
type WSMessage struct {
	Type string      `json:"type"` // Ej: "pregunta", "ataque", "resultado"
	Data interface{} `json:"data"`
}

// PlayerMessage es lo que el frontend (o Postman) nos envía al servidor
type PlayerMessage struct {
	Type string `json:"type"` // Puede ser "respuesta" o "ataque"
	Data string `json:"data"` // Puede ser "A", "B", "C", "D" o "Monstertify"
}
