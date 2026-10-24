package gateway

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"example.invalid/go/auth/credentials/idtoken"
)

func TestGoogleTokenVerifierReturnsVerifiedIdentity(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	var gotAudience string
	verifier := &GoogleTokenVerifier{validate: func(_ context.Context, token, audience string) (*idtoken.Payload, error) {
		if token != "token" {
			t.Fatalf("token = %q", token)
		}
		gotAudience = audience
		return &idtoken.Payload{Issuer: googleIssuer, IssuedAt: now.Add(-time.Minute).Unix(), Expires: now.Add(59 * time.Minute).Unix(), Subject: "subject-1", Claims: map[string]any{
			"email": "user@example.invalid", "email_verified": true,
		}}, nil
	}, now: func() time.Time { return now }}

	got, err := verifier.Verify(context.Background(), "token", "dbc-gateway")
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if got != (Identity{Subject: "subject-1", Email: "user@example.invalid"}) {
		t.Fatalf("Verify() = %#v", got)
	}
	if gotAudience != "dbc-gateway" {
		t.Fatalf("audience = %q", gotAudience)
	}
}

func TestGoogleTokenVerifierRejectsLibraryValidationFailure(t *testing.T) {
	verifier := &GoogleTokenVerifier{validate: func(context.Context, string, string) (*idtoken.Payload, error) {
		return nil, errors.New("bad signature")
	}}
	_, err := verifier.Verify(context.Background(), "token", "dbc-gateway")
	if err == nil || !strings.Contains(err.Error(), "驗證失敗") {
		t.Fatalf("Verify() error = %v", err)
	}
}

func TestGoogleTokenVerifierRequiresVerifiedEmail(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	for name, claims := range map[string]map[string]any{
		"missing email":    {"email_verified": true},
		"unverified email": {"email": "user@example.invalid", "email_verified": false},
		"wrong claim type": {"email": "user@example.invalid", "email_verified": "true"},
	} {
		t.Run(name, func(t *testing.T) {
			verifier := &GoogleTokenVerifier{validate: func(context.Context, string, string) (*idtoken.Payload, error) {
				return &idtoken.Payload{Issuer: googleIssuer, IssuedAt: now.Add(-time.Minute).Unix(), Expires: now.Add(59 * time.Minute).Unix(), Subject: "subject-1", Claims: claims}, nil
			}, now: func() time.Time { return now }}
			if _, err := verifier.Verify(context.Background(), "token", "dbc-gateway"); err == nil {
				t.Fatal("Verify() error = nil")
			}
		})
	}
}

func TestGoogleTokenVerifierRejectsInvalidIssuerAndTimeClaims(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	tests := map[string]*idtoken.Payload{
		"issuer":         {Issuer: "https://example.invalid", IssuedAt: now.Unix(), Expires: now.Add(time.Hour).Unix(), Subject: "subject-1"},
		"missing iat":    {Issuer: googleIssuer, Expires: now.Add(time.Hour).Unix(), Subject: "subject-1"},
		"future iat":     {Issuer: googleIssuer, IssuedAt: now.Add(tokenClockSkew + time.Second).Unix(), Expires: now.Add(time.Hour).Unix(), Subject: "subject-1"},
		"long lifetime":  {Issuer: googleIssuer, IssuedAt: now.Unix(), Expires: now.Add(maxTokenLifetime + time.Second).Unix(), Subject: "subject-1"},
		"inverted times": {Issuer: googleIssuer, IssuedAt: now.Unix(), Expires: now.Add(-time.Second).Unix(), Subject: "subject-1"},
	}
	for name, payload := range tests {
		t.Run(name, func(t *testing.T) {
			payload.Claims = map[string]any{"email": "user@example.invalid", "email_verified": true}
			verifier := &GoogleTokenVerifier{validate: func(context.Context, string, string) (*idtoken.Payload, error) {
				return payload, nil
			}, now: func() time.Time { return now }}
			if _, err := verifier.Verify(context.Background(), "token", "dbc-gateway"); err == nil {
				t.Fatal("Verify() error = nil")
			}
		})
	}
}
