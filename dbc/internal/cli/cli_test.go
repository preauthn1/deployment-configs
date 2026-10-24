package cli

import "testing"

func TestStartAddressFlagDefaultsToLoopback(t *testing.T) {
	cmd := newStartCmd("test")
	flag := cmd.Flags().Lookup("address")
	if flag == nil {
		t.Fatal("start command is missing --address flag")
	}
	if flag.DefValue != "0.0.0.0" {
		t.Fatalf("--address default = %q, want %q", flag.DefValue, "0.0.0.0")
	}

	if err := cmd.ParseFlags([]string{"--address", "0.0.0.0"}); err != nil {
		t.Fatalf("parse --address: %v", err)
	}
	if got := flag.Value.String(); got != "0.0.0.0" {
		t.Fatalf("parsed --address = %q, want %q", got, "0.0.0.0")
	}
}
