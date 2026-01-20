package controller

import (
	"net"
	"strings"
)

const (
	// Annotation keys
	AnnotationRegister     = "pihole.io/register"
	AnnotationTargetIP     = "pihole.io/target-ip"
	AnnotationHosts        = "pihole.io/hosts"
	AnnotationManagedHosts = "pihole.io/managed-hosts"

	// Finalizer name
	FinalizerName = "pihole.io/dns-cleanup"
)

// parseCommaSeparated parses a comma-separated string into a slice of trimmed strings
func parseCommaSeparated(s string) []string {
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// isValidIPv4 checks if the given string is a valid IPv4 address
func isValidIPv4(ip string) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	return parsed.To4() != nil
}
