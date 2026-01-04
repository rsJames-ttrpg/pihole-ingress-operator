package config

import (
	"os"
	"strings"
	"testing"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name    string
		envVars map[string]string
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			envVars: map[string]string{
				"PIHOLE_URL":        "http://192.168.1.2",
				"PIHOLE_PASSWORD":   "test-password",
				"DEFAULT_TARGET_IP": "192.168.1.100",
			},
			wantErr: false,
		},
		{
			name: "valid config with all options",
			envVars: map[string]string{
				"PIHOLE_URL":        "https://pihole.local",
				"PIHOLE_PASSWORD":   "test-password",
				"DEFAULT_TARGET_IP": "10.0.0.1",
				"LOG_LEVEL":         "debug",
				"WATCH_NAMESPACE":   "default",
			},
			wantErr: false,
		},
		{
			name: "missing PIHOLE_URL",
			envVars: map[string]string{
				"PIHOLE_PASSWORD":   "test-password",
				"DEFAULT_TARGET_IP": "192.168.1.100",
			},
			wantErr: true,
			errMsg:  "PIHOLE_URL is required",
		},
		{
			name: "missing PIHOLE_PASSWORD",
			envVars: map[string]string{
				"PIHOLE_URL":        "http://192.168.1.2",
				"DEFAULT_TARGET_IP": "192.168.1.100",
			},
			wantErr: true,
			errMsg:  "PIHOLE_PASSWORD is required",
		},
		{
			name: "missing DEFAULT_TARGET_IP",
			envVars: map[string]string{
				"PIHOLE_URL":      "http://192.168.1.2",
				"PIHOLE_PASSWORD": "test-password",
			},
			wantErr: true,
			errMsg:  "DEFAULT_TARGET_IP is required",
		},
		{
			name: "invalid PIHOLE_URL scheme",
			envVars: map[string]string{
				"PIHOLE_URL":        "ftp://192.168.1.2",
				"PIHOLE_PASSWORD":   "test-password",
				"DEFAULT_TARGET_IP": "192.168.1.100",
			},
			wantErr: true,
			errMsg:  "PIHOLE_URL must be an HTTP or HTTPS URL",
		},
		{
			name: "invalid DEFAULT_TARGET_IP",
			envVars: map[string]string{
				"PIHOLE_URL":        "http://192.168.1.2",
				"PIHOLE_PASSWORD":   "test-password",
				"DEFAULT_TARGET_IP": "not-an-ip",
			},
			wantErr: true,
			errMsg:  "DEFAULT_TARGET_IP is not a valid IPv4 address:",
		},
		{
			name: "IPv6 DEFAULT_TARGET_IP not allowed",
			envVars: map[string]string{
				"PIHOLE_URL":        "http://192.168.1.2",
				"PIHOLE_PASSWORD":   "test-password",
				"DEFAULT_TARGET_IP": "::1",
			},
			wantErr: true,
			errMsg:  "DEFAULT_TARGET_IP is not a valid IPv4 address:",
		},
		{
			name: "invalid LOG_LEVEL",
			envVars: map[string]string{
				"PIHOLE_URL":        "http://192.168.1.2",
				"PIHOLE_PASSWORD":   "test-password",
				"DEFAULT_TARGET_IP": "192.168.1.100",
				"LOG_LEVEL":         "invalid",
			},
			wantErr: true,
			errMsg:  "LOG_LEVEL must be one of: debug, info, warn, error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear environment
			os.Clearenv()

			// Set test environment variables
			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}

			cfg, err := Load()

			if tt.wantErr {
				if err == nil {
					t.Errorf("Load() expected error, got nil")
					return
				}
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Load() error = %q, want to contain %q", err.Error(), tt.errMsg)
				}
				return
			}

			if err != nil {
				t.Errorf("Load() unexpected error: %v", err)
				return
			}

			if cfg == nil {
				t.Error("Load() returned nil config")
				return
			}

			// Verify values
			if cfg.PiholeURL != tt.envVars["PIHOLE_URL"] {
				t.Errorf("PiholeURL = %q, want %q", cfg.PiholeURL, tt.envVars["PIHOLE_URL"])
			}
			if cfg.PiholePassword != tt.envVars["PIHOLE_PASSWORD"] {
				t.Errorf("PiholePassword = %q, want %q", cfg.PiholePassword, tt.envVars["PIHOLE_PASSWORD"])
			}
			if cfg.DefaultTargetIP != tt.envVars["DEFAULT_TARGET_IP"] {
				t.Errorf("DefaultTargetIP = %q, want %q", cfg.DefaultTargetIP, tt.envVars["DEFAULT_TARGET_IP"])
			}
		})
	}
}

func TestLoadDefaults(t *testing.T) {
	os.Clearenv()
	t.Setenv("PIHOLE_URL", "http://192.168.1.2")
	t.Setenv("PIHOLE_PASSWORD", "test-password")
	t.Setenv("DEFAULT_TARGET_IP", "192.168.1.100")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}

	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel default = %q, want %q", cfg.LogLevel, "info")
	}

	if cfg.WatchNamespace != "" {
		t.Errorf("WatchNamespace default = %q, want empty", cfg.WatchNamespace)
	}
}

func TestIsValidIPv4(t *testing.T) {
	tests := []struct {
		ip    string
		valid bool
	}{
		{"192.168.1.1", true},
		{"10.0.0.1", true},
		{"0.0.0.0", true},
		{"255.255.255.255", true},
		{"::1", false},
		{"fe80::1", false},
		{"not-an-ip", false},
		{"", false},
		{"192.168.1", false},
		{"192.168.1.256", false},
	}

	for _, tt := range tests {
		t.Run(tt.ip, func(t *testing.T) {
			got := isValidIPv4(tt.ip)
			if got != tt.valid {
				t.Errorf("isValidIPv4(%q) = %v, want %v", tt.ip, got, tt.valid)
			}
		})
	}
}
