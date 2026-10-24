package gateway

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestCloudIdentityGroupCheckerLooksUpThenChecksMembership(t *testing.T) {
	lookups := 0
	var checkedGroup, checkedMember string
	checker := &CloudIdentityGroupChecker{
		groupNames: make(map[string]string),
		lookupGroup: func(_ context.Context, group string) (string, error) {
			lookups++
			if group != "user@example.invalid" {
				t.Fatalf("group = %q", group)
			}
			return "groups/123", nil
		},
		checkMember: func(_ context.Context, group, member string) (bool, error) {
			checkedGroup, checkedMember = group, member
			return true, nil
		},
	}
	for range 2 {
		member, err := checker.IsMember(context.Background(), "user@example.invalid", "user@example.invalid")
		if err != nil || !member {
			t.Fatalf("IsMember() = %v, %v", member, err)
		}
	}
	if lookups != 1 {
		t.Fatalf("lookup count = %d, want 1", lookups)
	}
	if checkedGroup != "groups/123" || checkedMember != "user@example.invalid" {
		t.Fatalf("check args = (%q, %q)", checkedGroup, checkedMember)
	}
}

func TestCloudIdentityGroupCheckerFailsClosedOnAPIErrors(t *testing.T) {
	checker := &CloudIdentityGroupChecker{
		groupNames: make(map[string]string),
		lookupGroup: func(context.Context, string) (string, error) {
			return "", errors.New("permission denied")
		},
		checkMember: func(context.Context, string, string) (bool, error) {
			t.Fatal("checkMember must not run after lookup failure")
			return false, nil
		},
	}
	if _, err := checker.IsMember(context.Background(), "user@example.invalid", "user@example.invalid"); err == nil {
		t.Fatal("IsMember() error = nil")
	}
}

func TestCloudIdentityGroupCheckerPreflightResolvesUniqueGroups(t *testing.T) {
	var groups []string
	checker := &CloudIdentityGroupChecker{
		groupNames: make(map[string]string),
		lookupGroup: func(_ context.Context, group string) (string, error) {
			groups = append(groups, group)
			return "groups/" + group, nil
		},
		checkMember: func(context.Context, string, string) (bool, error) {
			t.Fatal("Preflight must not perform a membership check")
			return false, nil
		},
	}
	if err := checker.Preflight(context.Background(), []string{"user@example.invalid", "user@example.invalid", "user@example.invalid"}); err != nil {
		t.Fatalf("Preflight() error = %v", err)
	}
	if len(groups) != 2 || groups[0] != "user@example.invalid" || groups[1] != "user@example.invalid" {
		t.Fatalf("looked up groups = %v", groups)
	}
}

func TestCloudIdentityGroupCheckerPreflightFailsClosed(t *testing.T) {
	checker := &CloudIdentityGroupChecker{
		groupNames: make(map[string]string),
		lookupGroup: func(context.Context, string) (string, error) {
			return "", errors.New("permission denied")
		},
		checkMember: func(context.Context, string, string) (bool, error) { return false, nil },
	}
	if err := checker.Preflight(context.Background(), []string{"user@example.invalid"}); err == nil {
		t.Fatal("Preflight() error = nil")
	}
}

func TestEscapeCELString(t *testing.T) {
	got := escapeCELString(`user'\\@example.invalid`)
	if got != `user\'\\\\@example.invalid` {
		t.Fatalf("escapeCELString() = %q", got)
	}
	if strings.Contains(got, "' ||") {
		t.Fatalf("escaped value contains injectable expression: %q", got)
	}
}
