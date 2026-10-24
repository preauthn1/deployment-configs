package gateway

import (
	"net"
	"sync"
)

// connectionRegistry 保存目前存活的 client 與 backend connections，讓 forced
// drain 能連同卡在 backend 認證交握（例如 backend 中途無回應）的連線一起強制
// 關閉，而不只是關閉 client 端——兩者在 relay 階段開始前並未互相牽動，光關
// client 端無法解除卡在 backend 讀寫的 goroutine。以 Server 為 key，讓多個
// 本機測試 server 不互相影響。
var connectionRegistries sync.Map // map[*Server]*connectionRegistry

type connectionRegistry struct {
	mu    sync.Mutex
	conns map[net.Conn]struct{}
}

func (s *Server) connectionRegistry() *connectionRegistry {
	value, _ := connectionRegistries.LoadOrStore(s, &connectionRegistry{conns: make(map[net.Conn]struct{})})
	return value.(*connectionRegistry)
}

func (s *Server) trackConnection(conn net.Conn) {
	registry := s.connectionRegistry()
	registry.mu.Lock()
	registry.conns[conn] = struct{}{}
	registry.mu.Unlock()
}

func (s *Server) untrackConnection(conn net.Conn) {
	registry := s.connectionRegistry()
	registry.mu.Lock()
	delete(registry.conns, conn)
	registry.mu.Unlock()
}

func (s *Server) closeActiveConnections() {
	registry := s.connectionRegistry()
	registry.mu.Lock()
	connections := make([]net.Conn, 0, len(registry.conns))
	for conn := range registry.conns {
		connections = append(connections, conn)
	}
	registry.mu.Unlock()
	for _, conn := range connections {
		_ = conn.Close()
	}
}
