package hub

import (
	"encoding/json"
	"sync"
)

// Message is sent over WebSocket to clients.
type Message struct {
	Type  string          `json:"type"`            // "tick" | "event" | "connected" | "idle"
	Tick  int64           `json:"tick,omitempty"`
	Event json.RawMessage `json:"event,omitempty"`
}

// Hub manages WebSocket client connections and broadcasts messages.
type Hub struct {
	mu      sync.RWMutex
	clients map[string][]chan Message // accountID → channels (multiple tabs)
}

// New creates a new Hub.
func New() *Hub {
	return &Hub{
		clients: make(map[string][]chan Message),
	}
}

// Register adds a new connection channel for the given accountID and returns it.
func (h *Hub) Register(accountID string) chan Message {
	ch := make(chan Message, 64)
	h.mu.Lock()
	h.clients[accountID] = append(h.clients[accountID], ch)
	h.mu.Unlock()
	return ch
}

// Unregister removes a connection channel for the given accountID.
func (h *Hub) Unregister(accountID string, ch chan Message) {
	h.mu.Lock()
	defer h.mu.Unlock()
	chs := h.clients[accountID]
	for i, c := range chs {
		if c == ch {
			h.clients[accountID] = append(chs[:i], chs[i+1:]...)
			break
		}
	}
	if len(h.clients[accountID]) == 0 {
		delete(h.clients, accountID)
	}
	close(ch)
}

// SendToAgent sends a message to all connections belonging to a specific accountID.
func (h *Hub) SendToAgent(accountID string, msg Message) {
	h.mu.RLock()
	chs := h.clients[accountID]
	h.mu.RUnlock()
	for _, ch := range chs {
		select {
		case ch <- msg:
		default:
			// Drop if buffer full; client is too slow.
		}
	}
}

// Broadcast sends a message to all connected clients.
func (h *Hub) Broadcast(msg Message) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, chs := range h.clients {
		for _, ch := range chs {
			select {
			case ch <- msg:
			default:
			}
		}
	}
}

// ClientCount returns the total number of open WebSocket channels across all accounts.
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	n := 0
	for _, chs := range h.clients {
		n += len(chs)
	}
	return n
}

// ConnectedIDs returns all currently connected accountIDs (deduplicated, one entry per account).
func (h *Hub) ConnectedIDs() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	ids := make([]string, 0, len(h.clients))
	for id := range h.clients {
		ids = append(ids, id)
	}
	return ids
}

// KickClient closes all WebSocket connections for the given accountID.
func (h *Hub) KickClient(accountID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, ch := range h.clients[accountID] {
		close(ch)
	}
	delete(h.clients, accountID)
}
