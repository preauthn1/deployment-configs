package gateway

import "net"

// newTestServer builds a Server whose MySQL-authentication-termination and
// relay steps are no-ops, for tests that exercise the gatewayproto
// handshake/authorization/dial wiring without needing a real (or
// hand-simulated) MySQL protocol exchange on either side — mysqlauth_test.go
// covers the authentication logic itself directly.
func newTestServer(cfg *Config, authorizer *Authorizer) *Server {
	server := NewServer(cfg, authorizer)
	server.AuthenticateClient = func(net.Conn, net.Conn) error { return nil }
	server.AuthenticateBackend = func(net.Conn, string, string) error { return nil }
	server.Relay = func(client, backend net.Conn, _ *Authorization) error {
		relayBothWays(client, backend)
		return nil
	}
	return server
}
