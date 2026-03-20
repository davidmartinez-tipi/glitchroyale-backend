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
