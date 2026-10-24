package gateway

import (
	"net"
	"testing"
	"time"
)

func TestCloseActiveConnections(t *testing.T) {
	server := &Server{}
	client, peer := net.Pipe()
	server.trackConnection(client)
	server.closeActiveConnections()
	defer peer.Close()
	if _, err := peer.Write([]byte("x")); err == nil {
		t.Fatal("connection remained open after forced drain")
	}
	server.untrackConnection(client)
}

func TestWaitForDrainWaitsForRelayAfterForcedClose(t *testing.T) {
	server := &Server{Config: &Config{DrainTimeout: time.Millisecond}}
	client, peer := net.Pipe()
	defer peer.Close()
	server.trackConnection(client)
	server.active.Add(1)

	finished := make(chan struct{})
	go func() {
		defer close(finished)
		defer server.active.Done()
		defer server.untrackConnection(client)
		var buf [1]byte
		_, err := client.Read(buf[:])
		if err == nil {
			t.Error("Read() error = nil after forced close")
		}
		// Model relay cleanup that must finish before audit logging is closed.
		time.Sleep(20 * time.Millisecond)
	}()

	err := server.waitForDrain()
	if err == nil {
		t.Fatal("waitForDrain() error = nil, want drain timeout")
	}
	select {
	case <-finished:
	default:
		t.Fatal("waitForDrain returned before relay cleanup completed")
	}
}
