package app

import "sync"

const (
	maxChatConnectionsPerUser = 4
	maxChatConnectionsPerIP   = 64
)

type chatConnectionLimiter struct {
	mu     sync.Mutex
	byUser map[int64]int
	byIP   map[string]int
}

func newChatConnectionLimiter() *chatConnectionLimiter {
	return &chatConnectionLimiter{
		byUser: make(map[int64]int),
		byIP:   make(map[string]int),
	}
}

func (l *chatConnectionLimiter) acquire(userID int64, clientIP string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.byUser[userID] >= maxChatConnectionsPerUser || l.byIP[clientIP] >= maxChatConnectionsPerIP {
		return false
	}
	l.byUser[userID]++
	l.byIP[clientIP]++
	return true
}

func (l *chatConnectionLimiter) release(userID int64, clientIP string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if count := l.byUser[userID]; count <= 1 {
		delete(l.byUser, userID)
	} else {
		l.byUser[userID] = count - 1
	}
	if count := l.byIP[clientIP]; count <= 1 {
		delete(l.byIP, clientIP)
	} else {
		l.byIP[clientIP] = count - 1
	}
}
