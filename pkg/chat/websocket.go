package chat

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// WebSocket upgrader configuration
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// TODO: Implement proper origin checking in production
		return true
	},
}

// MessageType defines the type of WebSocket message
type MessageType string

const (
	// Client to server message types
	MessageTypeChat    MessageType = "chat"
	MessageTypePing    MessageType = "ping"
	MessageTypeHistory MessageType = "history"

	// Server to client message types
	MessageTypeResponse  MessageType = "response"
	MessageTypeError     MessageType = "error"
	MessageTypePong      MessageType = "pong"
	MessageTypeStreaming MessageType = "streaming"
	MessageTypeCitation  MessageType = "citation"
)

// Message represents a WebSocket message
type Message struct {
	Type      MessageType     `json:"type"`
	ID        string          `json:"id"`
	Content   string          `json:"content,omitempty"`
	Error     string          `json:"error,omitempty"`
	Metadata  json.RawMessage `json:"metadata,omitempty"`
	Timestamp time.Time       `json:"timestamp"`
}

// ChatMessage represents a chat message from the client
type ChatMessage struct {
	Query            string `json:"query"`
	ConversationID   string `json:"conversation_id,omitempty"`
	IncludeCitations bool   `json:"include_citations,omitempty"`
}

// StreamingResponse represents a streaming response chunk
type StreamingResponse struct {
	Chunk     string `json:"chunk"`
	Done      bool   `json:"done"`
	MessageID string `json:"message_id"`
}

// CitationResponse represents document citations
type CitationResponse struct {
	MessageID string     `json:"message_id"`
	Citations []Citation `json:"citations"`
}

// Citation represents a single citation
type Citation struct {
	DocumentID string                 `json:"document_id"`
	Content    string                 `json:"content"`
	Score      float64                `json:"score"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// Client represents a WebSocket client connection
type Client struct {
	ID        string
	conn      *websocket.Conn
	send      chan Message
	hub       *Hub
	ctx       context.Context
	cancel    context.CancelFunc
	connected bool
	mu        sync.RWMutex
}

// Hub manages WebSocket clients
type Hub struct {
	clients     map[string]*Client
	broadcast   chan Message
	register    chan *Client
	unregister  chan *Client
	chatService *Service
	mu          sync.RWMutex
}

// NewHub creates a new Hub instance
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[string]*Client),
		broadcast:  make(chan Message),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

// Run starts the hub's main loop
func (h *Hub) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			// Close all client connections
			h.mu.RLock()
			for _, client := range h.clients {
				close(client.send)
			}
			h.mu.RUnlock()
			return

		case client := <-h.register:
			h.mu.Lock()
			h.clients[client.ID] = client
			h.mu.Unlock()
			log.Printf("Client %s connected", client.ID)

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client.ID]; ok {
				delete(h.clients, client.ID)
				close(client.send)
				h.mu.Unlock()
				log.Printf("Client %s disconnected", client.ID)
			} else {
				h.mu.Unlock()
			}

		case message := <-h.broadcast:
			h.mu.RLock()
			for _, client := range h.clients {
				select {
				case client.send <- message:
				default:
					// Client's send channel is full, close it
					close(client.send)
					delete(h.clients, client.ID)
				}
			}
			h.mu.RUnlock()
		}
	}
}

// ServeWS handles WebSocket requests from clients
func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	// Create new client
	clientID := r.Header.Get("X-Client-ID")
	if clientID == "" {
		clientID = generateClientID()
	}

	// Create context for this client
	ctx, cancel := context.WithCancel(context.Background())

	client := &Client{
		ID:        clientID,
		conn:      conn,
		send:      make(chan Message, 256),
		hub:       h,
		ctx:       ctx,
		cancel:    cancel,
		connected: true,
	}

	// Register client
	h.register <- client

	// Start client routines
	go client.writePump()
	go client.readPump()
}

// readPump reads messages from the WebSocket connection
func (c *Client) readPump() {
	defer func() {
		c.SetConnected(false)
		c.cancel() // Cancel the client's context
		c.hub.unregister <- c
		c.conn.Close()
	}()

	_ = c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		var msg Message
		err := c.conn.ReadJSON(&msg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		// Process message based on type
		switch msg.Type {
		case MessageTypePing:
			c.send <- Message{
				Type:      MessageTypePong,
				ID:        msg.ID,
				Timestamp: time.Now(),
			}

		case MessageTypeChat:
			// Handle chat message through the service
			if c.hub.chatService != nil {
				go c.hub.chatService.HandleChatMessage(c.ctx, c, msg)
			} else {
				log.Printf("Chat service not initialized")
			}

		case MessageTypeHistory:
			// Handle conversation history request
			log.Printf("History request from %s", c.ID)
		}
	}
}

// writePump writes messages to the WebSocket connection
func (c *Client) writePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteJSON(message); err != nil {
				return
			}

		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// SendMessage sends a message to a specific client
func (h *Hub) SendMessage(clientID string, message Message) error {
	h.mu.RLock()
	client, ok := h.clients[clientID]
	h.mu.RUnlock()

	if !ok {
		return websocket.ErrCloseSent
	}

	select {
	case client.send <- message:
		return nil
	default:
		return websocket.ErrCloseSent
	}
}

// BroadcastMessage sends a message to all connected clients
func (h *Hub) BroadcastMessage(message Message) {
	h.broadcast <- message
}

// generateClientID generates a unique client ID
func generateClientID() string {
	return time.Now().Format("20060102150405") + "-" + generateRandomString(8)
}

// generateRandomString generates a random string of specified length
func generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	for i := range result {
		result[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}
	return string(result)
}

// SetChatService sets the chat service for the hub
func (h *Hub) SetChatService(service *Service) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.chatService = service
}

// IsConnected returns true if the client is still connected
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// SetConnected updates the client's connection status
func (c *Client) SetConnected(connected bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.connected = connected
}
