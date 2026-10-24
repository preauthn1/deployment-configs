package proxy

import (
	"net"
	"testing"
	"time"
)

func TestRelayBidirectionalWaitsForBothDirections(t *testing.T) {
	client, clientPeer := net.Pipe()
	server, serverPeer := net.Pipe()
	defer clientPeer.Close()
	defer serverPeer.Close()

	secondFinished := make(chan struct{})
	relayBidirectional(client, server, func() {}, func() {
		var buf [1]byte
		_, _ = server.Read(buf[:])
		time.Sleep(20 * time.Millisecond)
		close(secondFinished)
	})

	select {
	case <-secondFinished:
	default:
		t.Fatal("relayBidirectional returned before the second direction completed")
	}
}
