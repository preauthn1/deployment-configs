package gateway

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"example.invalid/sre/dbc/internal/gatewayproto"
)

type deadlineRecordingConn struct {
	net.Conn
	mu        sync.Mutex
	deadlines []time.Time
}

func (c *deadlineRecordingConn) SetDeadline(deadline time.Time) error {
	c.mu.Lock()
	c.deadlines = append(c.deadlines, deadline)
	c.mu.Unlock()
	return c.Conn.SetDeadline(deadline)
}

func (c *deadlineRecordingConn) snapshotDeadlines() []time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]time.Time(nil), c.deadlines...)
}

func TestHealthHandlerReflectsReadiness(t *testing.T) {
	s := &Server{}
	for _, tc := range []struct {
		path  string
		ready bool
		want  int
	}{
		{path: "/livez", want: http.StatusOK},
		{path: "/readyz", ready: false, want: http.StatusServiceUnavailable},
		{path: "/readyz", ready: true, want: http.StatusOK},
	} {
		s.ready.Store(tc.ready)
		recorder := httptest.NewRecorder()
		s.HealthHandler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, tc.path, nil))
		if recorder.Code != tc.want {
			t.Fatalf("%s ready=%v status=%d, want %d", tc.path, tc.ready, recorder.Code, tc.want)
		}
	}
}

func TestHandleConnAuthenticatesThenRelays(t *testing.T) {
	authorizer, _ := testAuthorizer()
	cfg := authorizer.Config
	cfg.AuthTimeout = time.Second
	cfg.DialTimeout = time.Second
	s := newTestServer(cfg, authorizer)

	clientSide, serverSide := net.Pipe()
	backendForGateway, backendForTest := net.Pipe()
	s.DialContext = func(_ context.Context, network, address string) (net.Conn, error) {
		if network != "tcp" || address != "0.0.0.0:3306" {
			t.Fatalf("DialContext(%q, %q)", network, address)
		}
		return backendForGateway, nil
	}
	go s.handleConn(context.Background(), serverSide)

	if err := gatewayproto.WriteFrame(clientSide, gatewayproto.ClientAuth{Version: gatewayproto.Version, IDToken: [REDACTED]", Target: "db"}); err != nil {
		t.Fatalf("WriteFrame() error = %v", err)
	}
	var result gatewayproto.AuthResult
	if err := gatewayproto.ReadFrame(clientSide, &result); err != nil {
		t.Fatalf("ReadFrame() error = %v", err)
	}
	if !result.OK || result.SessionID == "" {
		t.Fatalf("auth result = %#v", result)
	}

	want := []byte("mysql-wire-packet")
	go func() { _, _ = clientSide.Write(want) }()
	got := make([]byte, len(want))
	if _, err := backendForTest.Read(got); err != nil {
		t.Fatalf("backend read error = %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("backend got %q, want %q", got, want)
	}
	_ = clientSide.Close()
	_ = backendForTest.Close()
}

func TestHandleConnBackendAuthTimesOutOnHungBackend(t *testing.T) {
	authorizer, _ := testAuthorizer()
	cfg := authorizer.Config
	cfg.AuthTimeout = time.Second
	cfg.DialTimeout = 200 * time.Millisecond
	s := newTestServer(cfg, authorizer)
	s.AuthenticateBackend = authenticateBackend // real implementation, not the test no-op

	clientSide, serverSide := net.Pipe()
	backendForGateway, backendForTest := net.Pipe()
	defer backendForTest.Close()
	s.DialContext = func(context.Context, string, string) (net.Conn, error) {
		return backendForGateway, nil
	}

	done := make(chan struct{})
	go func() {
		s.handleConn(context.Background(), serverSide)
		close(done)
	}()

	if err := gatewayproto.WriteFrame(clientSide, gatewayproto.ClientAuth{Version: gatewayproto.Version, IDToken: [REDACTED]", Target: "db"}); err != nil {
		t.Fatalf("WriteFrame() error = %v", err)
	}
	// backendForTest never writes anything back — simulates a backend that
	// accepts the TCP connection but stalls mid-handshake. Without a
	// deadline on the backend conn, this would block forever.
	var result gatewayproto.AuthResult
	if err := gatewayproto.ReadFrame(clientSide, &result); err != nil {
		t.Fatalf("ReadFrame() error = %v", err)
	}
	if result.OK || result.Error != "database backend unavailable" {
		t.Fatalf("auth result = %#v, want generic backend-unavailable error", result)
	}
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("handleConn did not return after backend auth deadline elapsed")
	}
	_ = clientSide.Close()
}

func TestCloseActiveConnectionsClosesTrackedBackendConnections(t *testing.T) {
	s := &Server{}
	backendForGateway, backendForTest := net.Pipe()
	defer backendForTest.Close()

	s.trackConnection(backendForGateway)
	readDone := make(chan error, 1)
	go func() {
		buf := make([]byte, 1)
		_, err := backendForGateway.Read(buf)
		readDone <- err
	}()

	s.closeActiveConnections()

	select {
	case err := <-readDone:
		if err == nil {
			t.Fatal("read on force-closed backend connection returned nil error")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("closeActiveConnections did not unblock a pending backend read")
	}
}

func TestHandleConnDoesNotDialDeniedTarget(t *testing.T) {
	authorizer, _ := testAuthorizer()
	cfg := authorizer.Config
	cfg.AuthTimeout = time.Second
	cfg.DialTimeout = time.Second
	s := newTestServer(cfg, authorizer)
	dialed := false
	s.DialContext = func(context.Context, string, string) (net.Conn, error) {
		dialed = true
		return nil, nil
	}

	clientSide, serverSide := net.Pipe()
	go s.handleConn(context.Background(), serverSide)
	if err := gatewayproto.WriteFrame(clientSide, gatewayproto.ClientAuth{Version: gatewayproto.Version, IDToken: [REDACTED]", Target: "unknown"}); err != nil {
		t.Fatalf("WriteFrame() error = %v", err)
	}
	var result gatewayproto.AuthResult
	if err := gatewayproto.ReadFrame(clientSide, &result); err != nil {
		t.Fatalf("ReadFrame() error = %v", err)
	}
	if result.OK || result.Error != "permission denied" {
		t.Fatalf("auth result = %#v", result)
	}
	if dialed {
		t.Fatal("denied request dialed backend")
	}
	_ = clientSide.Close()
}

func TestHandleConnRefreshesDeadlineForBackendDial(t *testing.T) {
	authorizer, _ := testAuthorizer()
	cfg := authorizer.Config
	cfg.AuthTimeout = time.Second
	cfg.DialTimeout = 3 * time.Second
	s := newTestServer(cfg, authorizer)

	clientSide, rawServerSide := net.Pipe()
	serverSide := &deadlineRecordingConn{Conn: rawServerSide}
	backendForGateway, backendForTest := net.Pipe()
	deadlinesAtDial := make(chan []time.Time, 1)
	relayStarted := make(chan struct{})
	s.DialContext = func(context.Context, string, string) (net.Conn, error) {
		deadlinesAtDial <- serverSide.snapshotDeadlines()
		return backendForGateway, nil
	}
	s.Relay = func(net.Conn, net.Conn, *Authorization) error {
		close(relayStarted)
		return nil
	}
	go s.handleConn(context.Background(), serverSide)

	if err := gatewayproto.WriteFrame(clientSide, gatewayproto.ClientAuth{Version: gatewayproto.Version, IDToken: [REDACTED]", Target: "db"}); err != nil {
		t.Fatalf("WriteFrame() error = %v", err)
	}
	var result gatewayproto.AuthResult
	if err := gatewayproto.ReadFrame(clientSide, &result); err != nil {
		t.Fatalf("ReadFrame() error = %v", err)
	}
	if !result.OK {
		t.Fatalf("auth result = %#v", result)
	}
	<-relayStarted

	deadlines := <-deadlinesAtDial
	if len(deadlines) < 2 || !deadlines[1].After(deadlines[0]) {
		t.Fatalf("deadlines at backend dial = %v, want refreshed dial deadline", deadlines)
	}
	deadlines = serverSide.snapshotDeadlines()
	if len(deadlines) < 3 || !deadlines[len(deadlines)-1].IsZero() {
		t.Fatalf("deadlines after auth response = %v, want final deadline cleared", deadlines)
	}
	_ = clientSide.Close()
	_ = backendForTest.Close()
}
