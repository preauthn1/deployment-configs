package proxy

import (
	"net"

	"example.invalid/sre/dbc/internal/config"
)

// RelayWithQueryAudit 讓 dbc-gateway 重用與 dbc start 完全相同的 MySQL wire
// protocol 稽核器。呼叫前必須先對 project 執行 CheckAuditLogging。
func RelayWithQueryAudit(client, server net.Conn, project, account, targetID string) error {
	inst := config.Instance{Project: project, Name: targetID, Host: targetID}
	aud, err := newQueryAuditor(project, account, inst)
	if err != nil {
		return err
	}

	relayBidirectional(client, server, func() {
		relayClientToServer(server, client, aud)
	}, func() {
		relayServerToClient(client, server, aud)
	})
	return nil
}

// RelayWithQueryAuditPreAuthenticated is like RelayWithQueryAudit, but for a
// connection where MySQL-level authentication on both sides already
// completed via dbc-gateway's own auth-termination logic (see
// internal/gateway/mysqlauth.go's package doc for why that exists at all).
// There's no handshake packet left in the stream to detect
// CLIENT_QUERY_ATTRIBUTES from, so this always assumes it's off — safe here
// because the gateway's fake client-facing handshake never grants that
// capability (see mysqlauth.go's fakeCapabilities doc for why a client's
// claim of it is tolerated but never honored).
func RelayWithQueryAuditPreAuthenticated(client, server net.Conn, project, account, targetID string) error {
	inst := config.Instance{Project: project, Name: targetID, Host: targetID}
	aud, err := newQueryAuditor(project, account, inst)
	if err != nil {
		return err
	}

	relayBidirectional(client, server, func() {
		relayClientToServerPreAuthenticated(server, client, aud)
	}, func() {
		relayServerToClient(client, server, aud)
	})
	return nil
}
