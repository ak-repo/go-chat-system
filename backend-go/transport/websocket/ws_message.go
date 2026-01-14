package websocket

import "encoding/json"

type ReceiverType string

const (
	ReceiverUser  ReceiverType = "user"
	ReceiverGroup ReceiverType = "group"
)

type WSMessage struct {
	Event        string          `json:"event"`
	SenderID     string          `json:"sender_id,omitempty"`
	ReceiverID   string          `json:"receiver_id,omitempty"`
	ReceiverType ReceiverType    `json:"receiver_type,omitempty"`
	Data         json.RawMessage `json:"data"`
}

// Event → routing & intent
// SenderID → injected by server only
// ReceiverID + ReceiverType → supports user + group
// Data → flexible payload per event
