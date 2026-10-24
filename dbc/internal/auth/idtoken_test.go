package auth

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

// TestIdentityTokenDoesNotPassAudienceToGcloud guards against reintroducing
// `--audiences`: that flag only works for service accounts (verified live
// against gcloud 582.0.0 with a real user account, which errors with
// "Invalid account type for `--audiences`. Requires valid service account."),
// so real users would get IdentityToken failing outright if this regresses.
// The audience parameter is still required non-empty (see
// TestIdentityTokenRequiresAudience) but deliberately unused for the actual
// command — see idtoken.go's doc comment for why.
func TestIdentityTokenDoesNotPassAudienceToGcloud(t *testing.T) {
	var gotName string
	var gotArgs []string
	runner := func(_ context.Context, name string, args ...string) ([]byte, error) {
		gotName = name
		gotArgs = append([]string(nil), args...)
		return []byte(" signed-token\n"), nil
	}
	got, err := identityToken(context.Background(), "https://example.invalid", runner)
	if err != nil {
		t.Fatalf("identityToken() error = %v", err)
	}
	if got != "signed-token" {
		t.Fatalf("identityToken() = %q", got)
	}
	wantArgs := []string{"auth", "print-identity-token", "--quiet"}
	if gotName != "gcloud" || !reflect.DeepEqual(gotArgs, wantArgs) {
		t.Fatalf("command = %q %#v, want gcloud %#v", gotName, gotArgs, wantArgs)
	}
}

func TestIdentityTokenReturnsCommandError(t *testing.T) {
	runner := func(context.Context, string, ...string) ([]byte, error) {
		return []byte("login required\n"), errors.New("exit status 1")
	}
	_, err := identityToken(context.Background(), "dbc-gateway", runner)
	if err == nil || !strings.Contains(err.Error(), "login required") {
		t.Fatalf("identityToken() error = %v", err)
	}
}

func TestIdentityTokenRequiresAudience(t *testing.T) {
	called := false
	_, err := identityToken(context.Background(), " ", func(context.Context, string, ...string) ([]byte, error) {
		called = true
		return nil, nil
	})
	if err == nil || called {
		t.Fatalf("identityToken() error = %v, called=%v", err, called)
	}
}
