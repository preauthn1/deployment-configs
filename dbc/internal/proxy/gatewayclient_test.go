package proxy

import (
	"testing"

	"example.invalid/sre/dbc/internal/config"
)

func TestGatewayClientConfigIncludesDBCVersion(t *testing.T) {
	inst := config.Instance{Gateway: &config.Gateway{
		Endpoint: "example.invalid:8443", ServerName: "example.invalid",
		Audience: "https://example.invalid", Target: "database",
	}}
	got := gatewayClientConfig(inst, "v1.2.3")
	if got.Endpoint != inst.Gateway.Endpoint || got.ServerName != inst.Gateway.ServerName ||
		got.Audience != inst.Gateway.Audience || got.Target != inst.Gateway.Target || got.ClientVersion != "v1.2.3" {
		t.Fatalf("gatewayClientConfig() = %+v", got)
	}
}
