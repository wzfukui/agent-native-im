package ws

import "github.com/wzfukui/agent-native-im/internal/model"

// WSMessage is the JSON envelope for all WebSocket communication.
type WSMessage struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

// Inbound types (client -> server):
//   message.send  - send a message
//   ping          - keepalive
//
// Outbound types (server -> client):
//   message.new    - new message delivered
//   stream.start   - bot started streaming
//   stream.delta   - streaming chunk
//   stream.end     - streaming complete
//   pong           - keepalive response
//   error          - error notification

type SendPayload struct {
	ConversationID int64               `json:"conversation_id"`
	StreamID       string              `json:"stream_id,omitempty"`
	StreamType     string              `json:"stream_type,omitempty"` // start, delta, end
	Layers         model.MessageLayers `json:"layers"`
}
