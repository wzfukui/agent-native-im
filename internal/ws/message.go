package ws

import "github.com/wzfukui/agent-native-im/internal/model"

// WSMessage is the JSON envelope for all WebSocket communication.
type WSMessage struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

// Inbound types (client -> server):
//   message.send   - send a message
//   task.cancel    - cancel a task/stream (legacy)
//   stream.cancel  - cancel an active stream (forwarded to other clients)
//   typing         - typing indicator
//   status.update  - entity status update
//   ping           - keepalive
//
// Outbound types (server -> client):
//   message.new      - new message delivered
//   message.progress - transient progress update (not persisted)
//   stream.start     - streaming started
//   stream.delta     - streaming chunk
//   stream.end       - streaming complete
//   stream.cancel    - stream cancellation (forwarded from another client)
//   task.cancel      - task cancellation broadcast
//   task.cancelled   - cancellation confirmation to sender
//   friend.request.created - friend request created or received
//   friend.request.updated - friend request status changed
//   notification.new - inbox item created
//   notification.read - inbox item marked read
//   notification.read_all - all inbox items marked read
//   pong             - keepalive response
//   error            - error notification

type SendPayload struct {
	ConversationID int64               `json:"conversation_id"`
	StreamID       string              `json:"stream_id,omitempty"`
	StreamType     string              `json:"stream_type,omitempty"` // start, delta, end
	ContentType    model.ContentType   `json:"content_type,omitempty"`
	Layers         model.MessageLayers `json:"layers"`
	Attachments    []model.Attachment  `json:"attachments,omitempty"`
	Mentions       []int64             `json:"mentions,omitempty"`
	ReplyTo        *int64              `json:"reply_to,omitempty"`
}
