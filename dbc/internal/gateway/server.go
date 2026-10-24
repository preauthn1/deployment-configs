package gateway

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"example.invalid/sre/dbc/internal/gatewayproto"
	"example.invalid/sre/dbc/internal/proxy"
)

// DialContextFunc 讓測試可以用記憶體連線取代真實 MySQL。
type DialContextFunc func(ctx context.Context, network, address string) (net.Conn, error)

// RelayFunc 在認證完成後轉送並稽核一個 MySQL session。
type RelayFunc func(client, backend net.Conn, authz *Authorization) error

// AuthenticateClientFunc 假扮 MySQL server 完成一次跟 client 的假認證交握
// ——見 mysqlauth.go 的套件說明。backend 是已經完成真實認證的後端連線：
// 如果 client 的 Handshake Response 有指定資料庫（CLIENT_CONNECT_WITH_DB），
// 會用它對 backend 送 COM_INIT_DB 幫忙選好。
type AuthenticateClientFunc func(client, backend net.Conn) error

// AuthenticateBackendFunc 用選定 role 的憑證，跟後端 MySQL 完成一次真正的
// 認證交握。
type AuthenticateBackendFunc func(backend net.Conn, user, password string) error

// Server 接受已完成 TLS 的連線，先驗證 dbc 認證 frame，再對 client／後端
// 各自完成 MySQL 認證交握（見 mysqlauth.go），最後轉送 MySQL bytes。
type Server struct {
	Config              *Config
	Authorizer          *Authorizer
	DialContext         DialContextFunc
	AuthenticateClient  AuthenticateClientFunc
	AuthenticateBackend AuthenticateBackendFunc
	Relay               RelayFunc

	ready  atomic.Bool
	active sync.WaitGroup
}

// NewServer 建立 gateway server。呼叫 Serve 後才會進入 ready。
func NewServer(cfg *Config, authorizer *Authorizer) *Server {
	dialer := &net.Dialer{Timeout: cfg.DialTimeout}
	return &Server{
		Config: cfg, Authorizer: authorizer, DialContext: dialer.DialContext,
		AuthenticateClient:  authenticateClient,
		AuthenticateBackend: authenticateBackend,
		Relay: func(client, backend net.Conn, authz *Authorization) error {
			return proxy.RelayWithQueryAuditPreAuthenticated(client, backend, authz.Target.AuditProject, authz.Identity.Email, authz.TargetID)
		},
	}
}

// Ready 回報是否仍接受新連線。
func (s *Server) Ready() bool { return s != nil && s.ready.Load() }

// HealthHandler 提供不經公開 Gateway 的 /livez 與 /readyz。
func (s *Server) HealthHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/livez", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		if !s.Ready() {
			http.Error(w, "not ready", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	return mux
}

// Serve 阻塞接受連線。ctx 取消時先切掉 readiness、停止 accept，再等待現有
// session 至 drain timeout。listener 應由呼叫端包成 tls.Listener。
func (s *Server) Serve(ctx context.Context, listener net.Listener) error {
	if s == nil || s.Config == nil || s.Authorizer == nil || s.DialContext == nil || s.Relay == nil {
		return fmt.Errorf("dbc-gateway server 未完整初始化")
	}
	s.ready.Store(true)
	defer s.ready.Store(false)

	go func() {
		<-ctx.Done()
		s.ready.Store(false)
		_ = listener.Close()
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return s.waitForDrain()
			}
			var temporary interface{ Temporary() bool }
			if errors.As(err, &temporary) && temporary.Temporary() {
				continue
			}
			return fmt.Errorf("接受 dbc-gateway 連線失敗: %w", err)
		}
		if !s.Ready() {
			_ = conn.Close()
			continue
		}
		s.active.Add(1)
		s.trackConnection(conn)
		go func() {
			defer s.active.Done()
			defer s.untrackConnection(conn)
			s.handleConn(ctx, conn)
		}()
	}
}

func (s *Server) waitForDrain() error {
	done := make(chan struct{})
	go func() {
		s.active.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-time.After(s.Config.DrainTimeout):
		s.closeActiveConnections()
		// Closing every client connection unblocks the relay goroutines. Wait
		// until their deferred backend closes and audit calls have completed
		// before the caller flushes and closes the shared logging clients.
		<-done
		return fmt.Errorf("等待既有 session 結束超過 %s", s.Config.DrainTimeout)
	}
}

func (s *Server) handleConn(parent context.Context, client net.Conn) {
	defer client.Close()
	authCtx, cancel := context.WithTimeout(parent, s.Config.AuthTimeout)
	defer cancel()
	_ = client.SetDeadline(time.Now().Add(s.Config.AuthTimeout))

	var request gatewayproto.ClientAuth
	if err := gatewayproto.ReadFrame(client, &request); err != nil {
		_ = gatewayproto.WriteFrame(client, gatewayproto.AuthResult{OK: false, Error: "invalid authentication request"})
		return
	}
	authz, err := s.Authorizer.Authorize(authCtx, request)
	if err != nil {
		_ = gatewayproto.WriteFrame(client, gatewayproto.AuthResult{OK: false, Error: publicAuthError(err)})
		return
	}
	cancel()
	// Authorization and backend dialing have independent timeout budgets.
	// Refresh the connection deadline so time spent checking the token and
	// groups does not consume the backend dial/response budget.
	if err := client.SetDeadline(time.Now().Add(s.Config.DialTimeout)); err != nil {
		return
	}

	dialCtx, dialCancel := context.WithTimeout(parent, s.Config.DialTimeout)
	backend, err := s.DialContext(dialCtx, "tcp", authz.Target.Address)
	dialCancel()
	if err != nil {
		// The client's own deadline was set to expire at roughly this same
		// instant (both started from the DialTimeout budget above), so
		// writing the response now would race it. Give the write its own
		// fresh short budget rather than relying on whatever's left.
		_ = client.SetDeadline(time.Now().Add(s.Config.AuthTimeout))
		_ = gatewayproto.WriteFrame(client, gatewayproto.AuthResult{OK: false, Error: "database backend unavailable"})
		return
	}
	defer backend.Close()
	s.trackConnection(backend)
	defer s.untrackConnection(backend)

	// Bound the backend handshake with the same DialTimeout budget the
	// client's own deadline already reflects for this window (line above).
	// Without this, a backend that accepts the TCP connection but stalls
	// mid-handshake (firewall black hole, overloaded server, network
	// partition) leaves this goroutine blocked forever — closing the
	// tracked client connection alone won't unblock a read/write against
	// backend, since the two aren't wired together until the relay phase.
	if err := backend.SetDeadline(time.Now().Add(s.Config.DialTimeout)); err != nil {
		_ = client.SetDeadline(time.Now().Add(s.Config.AuthTimeout))
		_ = gatewayproto.WriteFrame(client, gatewayproto.AuthResult{OK: false, Error: "database backend unavailable"})
		return
	}

	// Authenticate to the backend for real, with the role's stored
	// credential, before ever telling the client it's connected — a
	// misconfigured/rotated credential is a gateway-side operational
	// problem, not something to reveal to the client, so it gets the same
	// generic "database backend unavailable" the dial-failure path above
	// uses.
	if err := s.AuthenticateBackend(backend, authz.Role.MySQLUser, authz.Role.MySQLPassword); err != nil {
		log.Printf("⚠️ 對後端認證失敗（target %s, role user %s): %v", authz.TargetID, authz.Role.MySQLUser, err)
		// backend.SetDeadline above shares the same DialTimeout budget as
		// the client's deadline (set before the dial, further up), so by
		// the time a backend-side timeout fires, the client's deadline has
		// also just about expired — refresh it before writing, or this
		// response races its own deadline and never reaches the client.
		_ = client.SetDeadline(time.Now().Add(s.Config.AuthTimeout))
		_ = gatewayproto.WriteFrame(client, gatewayproto.AuthResult{OK: false, Error: "database backend unavailable"})
		return
	}
	if err := backend.SetDeadline(time.Time{}); err != nil {
		_ = client.SetDeadline(time.Now().Add(s.Config.AuthTimeout))
		_ = gatewayproto.WriteFrame(client, gatewayproto.AuthResult{OK: false, Error: "database backend unavailable"})
		return
	}

	sessionID, err := newSessionID()
	if err != nil {
		_ = gatewayproto.WriteFrame(client, gatewayproto.AuthResult{OK: false, Error: "gateway internal error"})
		return
	}
	if err := gatewayproto.WriteFrame(client, gatewayproto.AuthResult{OK: true, SessionID: sessionID}); err != nil {
		return
	}
	if err := client.SetDeadline(time.Time{}); err != nil {
		return
	}

	// From here on the client expects raw MySQL protocol bytes, starting
	// with a server Handshake — fake one, see mysqlauth.go's package doc
	// for why the client's real credentials are never involved.
	if err := s.AuthenticateClient(client, backend); err != nil {
		log.Printf("⚠️ 假扮 client-facing MySQL handshake失敗: %v", err)
		return
	}

	_ = s.Relay(client, backend, authz)
}

func publicAuthError(err error) string {
	if errors.Is(err, ErrPermissionDenied) {
		return "permission denied"
	}
	return "authentication failed"
}

func newSessionID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw[:]), nil
}

func relayBothWays(client, backend net.Conn) {
	done := make(chan struct{}, 2)
	copyOne := func(dst, src net.Conn) {
		_, _ = io.Copy(dst, src)
		if tcp, ok := dst.(*net.TCPConn); ok {
			_ = tcp.CloseWrite()
		}
		done <- struct{}{}
	}
	go copyOne(backend, client)
	go copyOne(client, backend)
	<-done
}
