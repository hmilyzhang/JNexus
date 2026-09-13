// JNexus Ops Platform — By JJ Zhang, Version 1.0

package ws

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"jnexus/internal/pkg"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type client struct {
	conn  *websocket.Conn
	topic string
	send  chan []byte
}

type Hub struct {
	mu      sync.RWMutex
	clients map[string]map[*client]bool
}

var H = &Hub{clients: map[string]map[*client]bool{}}

func (h *Hub) subscribe(c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.clients[c.topic] == nil {
		h.clients[c.topic] = map[*client]bool{}
	}
	h.clients[c.topic][c] = true
}

func (h *Hub) unsubscribe(c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if set, ok := h.clients[c.topic]; ok {
		delete(set, c)
		if len(set) == 0 {
			delete(h.clients, c.topic)
		}
	}
	close(c.send)
}

func (h *Hub) Broadcast(topic string, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients[topic] {
		select {
		case c.send <- data:
		default: // drop when the send queue is full, to avoid blocking the execution engine
		}
	}
}

// Message structure: task push / terminal output
type Message struct {
	Type string          `json:"type"` // output / status / data / closed / error
	Data json.RawMessage `json:"data,omitempty"`
	Text string          `json:"text,omitempty"`
}

// TopicAuth authenticates, upgrades the connection, and attaches it to the topic
func Serve(c *gin.Context, topic string) {
	_, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	cl := &client{conn: conn, topic: topic, send: make(chan []byte, 256)}
	H.subscribe(cl)

	go writePump(cl)
	readPump(cl)
}

func writePump(c *client) {
	defer func() {
		c.conn.Close()
	}()
	for msg := range c.send {
		if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			return
		}
	}
}

func readPump(c *client) {
	defer func() {
		H.unsubscribe(c)
		c.conn.Close()
	}()
	for {
		if _, _, err := c.conn.ReadMessage(); err != nil {
			return
		}
	}
}

// For gin routes: WS entry point with JWT validation
func Handler(topicFactory func(c *gin.Context) string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, err := c.Cookie("token"); err == nil {
			// Support cookie-based auth
		} else if h := c.GetHeader("Sec-WebSocket-Protocol"); h != "" {
			c.Request.Header.Set("Authorization", "Bearer "+h)
		}
		// Parse the token manually (WS cannot always carry headers)
		token := c.Query("token")
		if token == "" {
			auth := c.GetHeader("Authorization")
			if len(auth) > 7 && auth[:7] == "Bearer " {
				token = auth[7:]
			}
		}
		if token != "" {
			if claims, err := pkg.ParseToken(token); err == nil {
				c.Set("user", claims)
			}
		}
		Serve(c, topicFactory(c))
	}
}

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}
