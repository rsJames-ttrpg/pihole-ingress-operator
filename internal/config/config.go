package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
)

// Config holds operator configuration
type Config struct {
	PiholeURL       string
	PiholePassword  string
	DefaultTargetIP string
	LogLevel        string
	WatchNamespace  string
}

// Load reads configuration from environment variables and validates it
func Load() (*Config, error) {
	cfg := &Config{
		PiholeURL:       os.Getenv("PIHOLE_URL"),
		PiholePassword:  os.Getenv("PIHOLE_PASSWORD"),
		DefaultTargetIP: os.Getenv("DEFAULT_TARGET_IP"),
		LogLevel:        os.Getenv("LOG_LEVEL"),
		WatchNamespace:  os.Getenv("WATCH_NAMESPACE"),
	}

	// Set defaults
	if cfg.LogLevel == "" {
		cfg.LogLevel = "info"
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Validate checks that all required configuration is present and valid
func (c *Config) Validate() error {
	// Validate PIHOLE_URL
	if c.PiholeURL == "" {
		return fmt.Errorf("PIHOLE_URL is required")
	}
	parsedURL, err := url.Parse(c.PiholeURL)
	if err != nil {
		return fmt.Errorf("PIHOLE_URL is not a valid URL: %w", err)
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("PIHOLE_URL must be an HTTP or HTTPS URL")
	}

	// Validate PIHOLE_PASSWORD
	if c.PiholePassword == "" {
		return fmt.Errorf("PIHOLE_PASSWORD is required")
	}

	// Validate DEFAULT_TARGET_IP
	if c.DefaultTargetIP == "" {
		return fmt.Errorf("DEFAULT_TARGET_IP is required")
	}
	if !isValidIPv4(c.DefaultTargetIP) {
		return fmt.Errorf("DEFAULT_TARGET_IP is not a valid IPv4 address: %s", c.DefaultTargetIP)
	}

	// Validate LOG_LEVEL
	validLogLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
	}
	if !validLogLevels[strings.ToLower(c.LogLevel)] {
		return fmt.Errorf("LOG_LEVEL must be one of: debug, info, warn, error")
	}
	c.LogLevel = strings.ToLower(c.LogLevel)

	return nil
}

// isValidIPv4 checks if the given string is a valid IPv4 address
func isValidIPv4(ip string) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	// Check it's IPv4 (not IPv6)
	return parsed.To4() != nil
}
