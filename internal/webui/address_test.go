package webui

import (
	"testing"

	"github.com/zedage/phorga/internal/config"
)

// Spec: WEB-001, WEB-006
func TestNewAddressPrecedence(t *testing.T) {
	tests := []struct {
		name     string
		flagAddr string
		webUI    *config.WebUIConfig
		want     string
	}{
		{"flag overrides config", "127.0.0.1:9090", &config.WebUIConfig{Enabled: true, Address: "127.0.0.1:8081"}, "127.0.0.1:9090"},
		{"config address", "", &config.WebUIConfig{Enabled: true, Address: "127.0.0.1:8081"}, "127.0.0.1:8081"},
		{"loopback default", "", nil, config.DefaultWebUIAddress},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := sampleConfig()
			cfg.WebUI = tc.webUI
			s, err := New(Options{Address: tc.flagAddr, ConfigPath: sampleConfigPath(), Config: cfg})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if s.Address() != tc.want {
				t.Errorf("address = %q, want %q", s.Address(), tc.want)
			}
		})
	}
	if config.DefaultWebUIAddress != "127.0.0.1:8080" {
		t.Errorf("default address = %q, must bind to loopback", config.DefaultWebUIAddress)
	}
}
