package main

import "sync"

type Metrics struct {
	MessagesSent  int64
	MessagesError int64
	BytesSent     int64
	mu            sync.RWMutex
}

func NewMetrics() *Metrics {
	return &Metrics{}
}

func (m *Metrics) IncrementSent(bytes int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.MessagesSent++
	m.BytesSent += int64(bytes)
}

func (m *Metrics) IncrementError() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.MessagesError++
}

func (m *Metrics) GetStats() (int64, int64, int64) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.MessagesSent, m.MessagesError, m.BytesSent
}
