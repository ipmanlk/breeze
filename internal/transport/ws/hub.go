package ws

import (
	"log/slog"
	"sync"

	"github.com/coder/websocket"
)

// ConnectionTracker manages user connection counts to properly handle
// multi-tab scenarios. A user is only "offline" when ALL their connections
// have disconnected.
type ConnectionTracker struct {
	mu sync.Mutex
	// userConnections maps userID -> count of active connections
	userConnections map[string]int
	// clientUser maps client pointer -> userID for cleanup
	clientUser map[*Client]string
}

func NewConnectionTracker() *ConnectionTracker {
	return &ConnectionTracker{
		userConnections: make(map[string]int),
		clientUser:      make(map[*Client]string),
	}
}

func (ct *ConnectionTracker) Add(client *Client) {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	ct.clientUser[client] = client.userID
	ct.userConnections[client.userID]++
}

func (ct *ConnectionTracker) Remove(client *Client) (userID string, count int) {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	userID = ct.clientUser[client]
	if userID == "" {
		return "", 0
	}
	delete(ct.clientUser, client)
	ct.userConnections[userID]--
	count = ct.userConnections[userID]
	if count <= 0 {
		delete(ct.userConnections, userID)
		count = 0
	}
	return userID, count
}

func (ct *ConnectionTracker) Count(userID string) int {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	return ct.userConnections[userID]
}

type Hub struct {
	clients         map[*Client]bool
	rooms           map[string]map[*Client]bool
	register        chan *registerAction
	unregister      chan unregisterAction
	disconnect      chan string
	subscribe       chan *subAction
	unsubscribe     chan *subAction
	broadcast       chan *roomMessage
	broadcastExcept chan *roomMessageExcept
	shutdown        chan struct{}
	shutdownOnce    sync.Once
	done            chan struct{}
	log             *slog.Logger
	tracker         *ConnectionTracker
	maxPerUser      int
	maxGlobal       int
}

type subAction struct {
	client  *Client
	roomKey string
}

// registerAction carries a client seeking registration. done receives true
// if the connection was accepted (under the per-user and global caps) or
// false if rejected, so the caller can tear down the rejected connection.
type registerAction struct {
	client *Client
	done   chan bool
}

type roomMessage struct {
	roomKey string
	data    []byte
}

type roomMessageExcept struct {
	roomKey      string
	data         []byte
	exceptUserID string
}

// unregisterAction carries the client to remove from the hub. When done is
// non-nil, the hub sends the remaining connection count (for that user after
// removal) on the channel and the caller blocks until it's processed.
type unregisterAction struct {
	client *Client
	done   chan int // if non-nil, remaining count sent after processing
}

// hubChannelBuffer is the buffer size for the Hub's internal channels
// (register, unregister, disconnect, subscribe, unsubscribe, broadcast).
const hubChannelBuffer = 256

func NewHub(log *slog.Logger) *Hub {
	return NewHubWithLimits(log, 0, 0)
}

// NewHubWithLimits creates a Hub enforcing per-user and global connection
// caps. A value of 0 disables the corresponding limit. Use this in
// production to bound resource use.
func NewHubWithLimits(log *slog.Logger, maxPerUser, maxGlobal int) *Hub {
	return &Hub{
		clients:         make(map[*Client]bool),
		rooms:           make(map[string]map[*Client]bool),
		register:        make(chan *registerAction, hubChannelBuffer),
		unregister:      make(chan unregisterAction, hubChannelBuffer),
		disconnect:      make(chan string, hubChannelBuffer),
		subscribe:       make(chan *subAction, hubChannelBuffer),
		unsubscribe:     make(chan *subAction, hubChannelBuffer),
		broadcast:       make(chan *roomMessage, hubChannelBuffer),
		broadcastExcept: make(chan *roomMessageExcept, hubChannelBuffer),
		shutdown:        make(chan struct{}),
		done:            make(chan struct{}),
		log:             log,
		tracker:         NewConnectionTracker(),
		maxPerUser:      maxPerUser,
		maxGlobal:       maxGlobal,
	}
}

func (h *Hub) Run() {
	for {
		exit := func() bool {
			defer func() {
				if r := recover(); r != nil {
					h.log.Error("hub panic recovered", "panic", r)
				}
			}()
			select {
			case action := <-h.register:
				c := action.client
				if h.rejectConnection(c) {
					h.log.Warn("websocket connection rejected: limit reached",
						"user_id", c.userID,
						"per_user", h.maxPerUser,
						"global", h.maxGlobal,
						"current_global", len(h.clients),
					)
					if action.done != nil {
						action.done <- false
					}
					return false
				}
				h.clients[c] = true
				h.tracker.Add(c)
				if action.done != nil {
					action.done <- true
				}

			case action := <-h.unregister:
				c := action.client
				if _, ok := h.clients[c]; ok {
					delete(h.clients, c)
					h.removeClientFromAllRooms(c)
					_, remaining := h.tracker.Remove(c)
					c.closeSend()
					if action.done != nil {
						action.done <- remaining
					}
				} else if action.done != nil {
					remaining := h.tracker.Count(c.userID)
					action.done <- remaining
				}

			case userID := <-h.disconnect:
				for c := range h.clients {
					if c.userID == userID {
						delete(h.clients, c)
						h.removeClientFromAllRooms(c)
						h.tracker.Remove(c)
						c.closeSend()
					}
				}

			case action := <-h.subscribe:
				if h.rooms[action.roomKey] == nil {
					h.rooms[action.roomKey] = make(map[*Client]bool)
				}
				h.rooms[action.roomKey][action.client] = true

			case action := <-h.unsubscribe:
				if clients, ok := h.rooms[action.roomKey]; ok {
					delete(clients, action.client)
					if len(clients) == 0 {
						delete(h.rooms, action.roomKey)
					}
				}

			case msg := <-h.broadcast:
				recipients := h.rooms[msg.roomKey]
				for client := range recipients {
					select {
					case client.send <- msg.data:
					default:
						h.log.Warn("client send buffer full, dropping message",
							"user_id", client.userID,
							"room", msg.roomKey,
						)
					}
				}
			case msg := <-h.broadcastExcept:
				recipients := h.rooms[msg.roomKey]
				for client := range recipients {
					if client.userID == msg.exceptUserID {
						continue
					}
					select {
					case client.send <- msg.data:
					default:
						h.log.Warn("client send buffer full, dropping broadcastExcept message",
							"user_id", client.userID,
							"room", msg.roomKey,
						)
					}
				}

			case <-h.shutdown:
				// Graceful shutdown: close every client so their ReadPump/WritePump
				// exit and the HTTP handlers complete cleanup. Tracker counts are
				// cleared too (on process exit the in-memory state is discarded;
				// this avoids double-counting if the process lingers).
				for c := range h.clients {
					delete(h.clients, c)
					h.removeClientFromAllRooms(c)
					h.tracker.Remove(c)
					c.closeSend()
					if c.conn != nil {
						c.conn.Close(websocket.StatusGoingAway, "server shutting down")
					}
				}
				close(h.done)
				return true
			}
			return false // unreachable: select blocks until a case fires
		}()
		if exit {
			return
		}
	}
}

func (h *Hub) Register(client *Client) bool {
	done := make(chan bool, 1)
	h.register <- &registerAction{client: client, done: done}
	return <-done
}

// rejectConnection reports whether a prospective connection would exceed the
// configured per-user or global connection caps. A limit of 0 means
// unlimited. Must be called from the Hub's Run goroutine (no locking needed).
func (h *Hub) rejectConnection(c *Client) bool {
	if h.maxPerUser > 0 && h.tracker.Count(c.userID) >= h.maxPerUser {
		return true
	}
	if h.maxGlobal > 0 && len(h.clients) >= h.maxGlobal {
		return true
	}
	return false
}

// Shutdown initiates a graceful shutdown: closes all client connections with a
// "going away" status so their HTTP handlers complete cleanup, then signals
// Done(). It is idempotent. Callers should also cancel the context used to
// start Run (or block on Done) so the Run goroutine exits. Safe to call from
// any goroutine.
func (h *Hub) Shutdown() {
	h.shutdownOnce.Do(func() {
		close(h.shutdown)
	})
}

// Done returns a channel that is closed once Shutdown has drained all
// connections and the Hub's Run loop has exited. Use it to wait for graceful
// shutdown completion.
func (h *Hub) Done() <-chan struct{} {
	return h.done
}

func (h *Hub) Unregister(client *Client) {
	h.unregister <- unregisterAction{client: client}
}

// UnregisterAndWait synchronously removes the client from the hub and returns
// the remaining connection count for that user. Blocks until the hub has
// processed the removal. Call this when the caller needs an accurate count
// of remaining connections before deciding e.g. whether to set presence offline.
func (h *Hub) UnregisterAndWait(client *Client) int {
	done := make(chan int, 1)
	h.unregister <- unregisterAction{client: client, done: done}
	return <-done
}

// GetConnectionCount returns the current connection count for a user
func (h *Hub) GetConnectionCount(userID string) int {
	return h.tracker.Count(userID)
}

func (h *Hub) DisconnectUser(userID string) {
	h.disconnect <- userID
}

func (h *Hub) Subscribe(client *Client, roomKey string) {
	h.subscribe <- &subAction{client: client, roomKey: roomKey}
}

func (h *Hub) Unsubscribe(client *Client, roomKey string) {
	h.unsubscribe <- &subAction{client: client, roomKey: roomKey}
}

func (h *Hub) Broadcast(roomKey string, eventType string, payload any) error {
	data, err := MarshalEvent(eventType, payload)
	if err != nil {
		return err
	}
	// Once the Run loop has exited (shutdown), nobody drains these channels:
	// a plain send would block the caller forever. Selecting on shutdown
	// turns post-shutdown publishes into a benign no-op.
	select {
	case h.broadcast <- &roomMessage{roomKey: roomKey, data: data}:
		return nil
	case <-h.shutdown:
		return nil
	}
}

// BroadcastExcept sends an event to all clients in the room EXCEPT the one
// with the given userID. Used to avoid echoing typing indicators back to the
// sender, for example.
func (h *Hub) BroadcastExcept(roomKey string, eventType string, payload any, exceptUserID string) error {
	data, err := MarshalEvent(eventType, payload)
	if err != nil {
		return err
	}
	select {
	case h.broadcastExcept <- &roomMessageExcept{roomKey: roomKey, data: data, exceptUserID: exceptUserID}:
		return nil
	case <-h.shutdown:
		return nil
	}
}

func (h *Hub) removeClientFromAllRooms(client *Client) {
	for roomKey, clients := range h.rooms {
		delete(clients, client)
		if len(clients) == 0 {
			delete(h.rooms, roomKey)
		}
	}
}
