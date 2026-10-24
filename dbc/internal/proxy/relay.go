package proxy

import "net"

// relayBidirectional runs both halves of a proxied connection. Once either
// direction ends, both connections are closed to unblock the other direction;
// the function does not return until both goroutines have completed.
func relayBidirectional(client, server net.Conn, clientToServer, serverToClient func()) {
	done := make(chan struct{}, 2)
	run := func(relay func()) {
		relay()
		done <- struct{}{}
	}
	go run(clientToServer)
	go run(serverToClient)

	<-done
	_ = client.Close()
	_ = server.Close()
	<-done
}
