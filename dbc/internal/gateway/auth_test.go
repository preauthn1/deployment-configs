package gateway

import (
	"context"
	"errors"
	"testing"

	"example.invalid/sre/dbc/internal/gatewayproto"
)

type fakeTokenVerifier struct {
	identity Identity
	err      error
	audience string
}

func (f *fakeTokenVerifier) Verify(_ context.Context, _ string, audience string) (Identity, error) {
	f.audience = audience
	return f.identity, f.err
}

type fakeGroupChecker struct {
	members map[string]bool
	err     error
}

func (f *fakeGroupChecker) IsMember(_ context.Context, email, group string) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	return f.members[email+"|"+group], nil
}

func testAuthorizer() (*Authorizer, *fakeTokenVerifier) {
	verifier := &fakeTokenVerifier{identity: Identity{Subject: "123", Email: "user@example.invalid"}}
	return &Authorizer{
		Config: &Config{Audience: "dbc-gateway", Targets: map[string]TargetConfig{
			"db": {Address: "0.0.0.0:3306", AuditProject: "logs", Roles: []TargetRole{
				{Groups: []string{"user@example.invalid"}, MySQLUser: "dbc_readonly", MySQLPassword: [REDACTED]"},
			}},
		}},
		TokenVerifier: verifier,
		GroupChecker:  &fakeGroupChecker{members: map[string]bool{"user@example.invalid|user@example.invalid": true}},
	}, verifier
}

func TestAuthorizeAllowsConfiguredGroup(t *testing.T) {
	a, verifier := testAuthorizer()
	got, err := a.Authorize(context.Background(), gatewayproto.ClientAuth{
		Version: gatewayproto.Version, IDToken: [REDACTED]", Target: "db",
	})
	if err != nil {
		t.Fatalf("Authorize() error = %v", err)
	}
	if got.Identity.Email != "user@example.invalid" || got.TargetID != "db" {
		t.Fatalf("Authorize() = %#v", got)
	}
	if got.Role.MySQLUser != "dbc_readonly" {
		t.Fatalf("Role.MySQLUser = %q, want dbc_readonly", got.Role.MySQLUser)
	}
	if verifier.audience != "dbc-gateway" {
		t.Fatalf("Verify audience = %q", verifier.audience)
	}
}

// TestAuthorizeUsesFirstMatchingRole covers the ordering guarantee
// TargetRole's doc promises: a caller who belongs to multiple roles' groups
// gets the first one listed, not the last — so config authors must put
// higher-privilege roles first.
func TestAuthorizeUsesFirstMatchingRole(t *testing.T) {
	verifier := &fakeTokenVerifier{identity: Identity{Subject: "123", Email: "user@example.invalid"}}
	a := &Authorizer{
		Config: &Config{Audience: "dbc-gateway", Targets: map[string]TargetConfig{
			"db": {Address: "0.0.0.0:3306", AuditProject: "logs", Roles: []TargetRole{
				{Groups: []string{"user@example.invalid"}, MySQLUser: "dbc_admin", MySQLPassword: [REDACTED]"},
				{Groups: []string{"user@example.invalid"}, MySQLUser: "dbc_readonly", MySQLPassword: [REDACTED]"},
			}},
		}},
		TokenVerifier: verifier,
		GroupChecker: &fakeGroupChecker{members: map[string]bool{
			"user@example.invalid|user@example.invalid":  true,
			"user@example.invalid|user@example.invalid": true,
		}},
	}

	got, err := a.Authorize(context.Background(), gatewayproto.ClientAuth{Version: gatewayproto.Version, IDToken: [REDACTED]", Target: "db"})
	if err != nil {
		t.Fatalf("Authorize() error = %v", err)
	}
	if got.Role.MySQLUser != "dbc_admin" {
		t.Fatalf("Role.MySQLUser = %q, want dbc_admin (the first matching role, listed first)", got.Role.MySQLUser)
	}
}

func TestAuthorizeRejectsUnknownTargetBeforeTokenVerification(t *testing.T) {
	a, verifier := testAuthorizer()
	_, err := a.Authorize(context.Background(), gatewayproto.ClientAuth{
		Version: gatewayproto.Version, IDToken: [REDACTED]", Target: "0.0.0.0:3306",
	})
	if !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("Authorize() error = %v, want permission denied", err)
	}
	if verifier.audience != "" {
		t.Fatal("unknown target must be rejected before external token verification")
	}
}

func TestAuthorizeRejectsInvalidToken(t *testing.T) {
	a, verifier := testAuthorizer()
	verifier.err = errors.New("bad signature")
	_, err := a.Authorize(context.Background(), gatewayproto.ClientAuth{Version: gatewayproto.Version, IDToken: [REDACTED]", Target: "db"})
	if !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("Authorize() error = %v, want unauthenticated", err)
	}
}

func TestAuthorizeFailsClosedWhenGroupLookupFails(t *testing.T) {
	a, _ := testAuthorizer()
	a.GroupChecker = &fakeGroupChecker{err: errors.New("api unavailable")}
	_, err := a.Authorize(context.Background(), gatewayproto.ClientAuth{Version: gatewayproto.Version, IDToken: [REDACTED]", Target: "db"})
	if err == nil || errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("Authorize() error = %v, want group service error", err)
	}
}

func TestAuthorizeRejectsNonMember(t *testing.T) {
	a, _ := testAuthorizer()
	a.GroupChecker = &fakeGroupChecker{members: map[string]bool{}}
	_, err := a.Authorize(context.Background(), gatewayproto.ClientAuth{Version: gatewayproto.Version, IDToken: [REDACTED]", Target: "db"})
	if !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("Authorize() error = %v, want permission denied", err)
	}
}
