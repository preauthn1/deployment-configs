package proxy

import (
	"net"
	"testing"
)

func TestListenAddress(t *testing.T) {
	tests := []struct {
		name    string
		address string
		port    int
		want    string
	}{
		{name: "default loopback", address: "0.0.0.0", port: 3510, want: "0.0.0.0:3510"},
		{name: "all IPv4 interfaces", address: "0.0.0.0", port: 3510, want: "0.0.0.0:3510"},
		{name: "IPv6", address: "::1", port: 3510, want: "[::1]:3510"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := listenAddress(tt.address, tt.port); got != tt.want {
				t.Fatalf("listenAddress(%q, %d) = %q, want %q", tt.address, tt.port, got, tt.want)
			}
		})
	}
}

func TestListenAddressCanBindAllIPv4Interfaces(t *testing.T) {
	ln, err := net.Listen("tcp", listenAddress("0.0.0.0", 0))
	if err != nil {
		t.Fatalf("listen on all IPv4 interfaces: %v", err)
	}
	defer ln.Close()

	addr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("listener address type = %T, want *net.TCPAddr", ln.Addr())
	}
	if !addr.IP.IsUnspecified() {
		t.Fatalf("listener IP = %s, want unspecified address", addr.IP)
	}
}
