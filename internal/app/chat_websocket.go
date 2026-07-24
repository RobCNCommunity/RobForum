package app

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
	"roblox-community/internal/domain"
)

const (
	chatWriteTimeout = 10 * time.Second
	chatPongTimeout  = 70 * time.Second
	chatPingInterval = 30 * time.Second
)

type chatHub struct {
	mu    sync.RWMutex
	rooms map[int64]map[*chatClient]struct{}
}

type chatClient struct {
	conversationID int64
	userID         int64
	conn           *websocket.Conn
	send           chan domain.Message
	closed         chan struct{}
	closeOnce      sync.Once
}

func newChatHub() *chatHub {
	return &chatHub{rooms: make(map[int64]map[*chatClient]struct{})}
}

func (h *chatHub) register(client *chatClient) {
	h.mu.Lock()
	defer h.mu.Unlock()
	room := h.rooms[client.conversationID]
	if room == nil {
		room = make(map[*chatClient]struct{})
		h.rooms[client.conversationID] = room
	}
	room[client] = struct{}{}
}

func (h *chatHub) unregister(client *chatClient) {
	h.mu.Lock()
	room := h.rooms[client.conversationID]
	if room != nil {
		delete(room, client)
		if len(room) == 0 {
			delete(h.rooms, client.conversationID)
		}
	}
	h.mu.Unlock()
	client.close()
}

func (h *chatHub) broadcast(message domain.Message) {
	h.mu.RLock()
	room := h.rooms[message.ConversationID]
	clients := make([]*chatClient, 0, len(room))
	for client := range room {
		clients = append(clients, client)
	}
	h.mu.RUnlock()
	for _, client := range clients {
		select {
		case client.send <- message:
		default:
			h.unregister(client)
		}
	}
}

func (h *chatHub) disconnectUser(conversationID, userID int64) {
	h.mu.RLock()
	room := h.rooms[conversationID]
	clients := make([]*chatClient, 0)
	for client := range room {
		if client.userID == userID {
			clients = append(clients, client)
		}
	}
	h.mu.RUnlock()
	for _, client := range clients {
		h.unregister(client)
	}
}

func (h *chatHub) disconnectConversation(conversationID int64) {
	h.mu.RLock()
	room := h.rooms[conversationID]
	clients := make([]*chatClient, 0, len(room))
	for client := range room {
		clients = append(clients, client)
	}
	h.mu.RUnlock()
	for _, client := range clients {
		h.unregister(client)
	}
}

func (c *chatClient) close() {
	c.closeOnce.Do(func() {
		close(c.closed)
		_ = c.conn.Close()
	})
}

func (c *chatClient) writePump() {
	ticker := time.NewTicker(chatPingInterval)
	defer ticker.Stop()
	for {
		select {
		case message := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(chatWriteTimeout))
			if err := c.conn.WriteJSON(message); err != nil {
				c.close()
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(chatWriteTimeout))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				c.close()
				return
			}
		case <-c.closed:
			return
		}
	}
}

func (s *Server) streamConversationMessages(w http.ResponseWriter, r *http.Request) {
	conversationID, _ := strconv.ParseInt(chi.URLParam(r, "conversationID"), 10, 64)
	user := currentUser(r)
	if conversationID < 1 {
		writeError(w, http.StatusNotFound, "conversation_not_found", "会话不存在")
		return
	}
	originPresent, originOK := sameOriginRequest(r, s.publicURL, s.trustedProxies)
	if !originPresent || !originOK {
		writeError(w, http.StatusForbidden, "websocket_origin_invalid", "实时连接来源无效")
		return
	}
	if _, err := s.store.GetConversation(user.ID, conversationID); err != nil {
		writeError(w, http.StatusForbidden, "messages_forbidden", err.Error())
		return
	}
	if s.chatLimit == nil {
		writeError(w, http.StatusServiceUnavailable, "websocket_unavailable", "实时连接暂不可用")
		return
	}
	clientIP := requestClientIP(r, s.trustedProxies)
	if !s.chatLimit.acquire(user.ID, clientIP) {
		writeError(w, http.StatusTooManyRequests, "websocket_connection_limit", "实时连接数量已达上限")
		return
	}
	defer s.chatLimit.release(user.ID, clientIP)
	upgrader := websocket.Upgrader{
		HandshakeTimeout: 10 * time.Second,
		CheckOrigin: func(request *http.Request) bool {
			present, valid := sameOriginRequest(request, s.publicURL, s.trustedProxies)
			return present && valid
		},
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	client := &chatClient{
		conversationID: conversationID,
		userID:         user.ID,
		conn:           conn,
		send:           make(chan domain.Message, 64),
		closed:         make(chan struct{}),
	}
	s.chatHub.register(client)
	defer s.chatHub.unregister(client)
	go client.writePump()

	conn.SetReadLimit(1024)
	_ = conn.SetReadDeadline(time.Now().Add(chatPongTimeout))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(chatPongTimeout))
	})
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
	}
}
