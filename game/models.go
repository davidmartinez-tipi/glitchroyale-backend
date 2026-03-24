package game

// Question representa los datos que enviaremos al frontend.
// NOTA: Omitimos deliberadamente "correct_option" para evitar trampas.

// WSMessage es el sobre que envuelve todos nuestros mensajes de WebSocket

type AttackInfo struct {
	Name     string
	Cost     int
	Damage   int
	Category string
}

// Mapa global con los datos oficiales del documento de diseño

type AttackRequest struct {
	TargetID string `json:"target_id"` // A quién atacamos
	Type     string `json:"type"`      // Qué ataque usamos (ej. "Monstertify")
}

// WSMessage es la estructura estándar para enviar mensajes por el WebSocket

// PlayerMessage es lo que el frontend (o Postman) nos envía al servidor
type PlayerMessage struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"` // 🔥 Cambiado de string a interface{}
}
