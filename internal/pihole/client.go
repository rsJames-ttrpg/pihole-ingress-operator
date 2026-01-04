package pihole

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Client interface for Pi-hole API operations
type Client interface {
	ListRecords(ctx context.Context) ([]DNSRecord, error)
	CreateRecord(ctx context.Context, record DNSRecord) error
	DeleteRecord(ctx context.Context, domain string) error
	Healthy(ctx context.Context) bool
}

// HTTPClient is a Pi-hole v6 API client using HTTP
type HTTPClient struct {
	baseURL    string
	password   string
	httpClient *http.Client

	// Session management
	mu    sync.RWMutex
	sid   string
	csrf  string
	valid time.Time
}

// NewClient creates a new Pi-hole API client
func NewClient(baseURL, password string) *HTTPClient {
	return &HTTPClient{
		baseURL:  strings.TrimSuffix(baseURL, "/"),
		password: password,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// authResponse represents the Pi-hole v6 auth response
type authResponse struct {
	Session struct {
		SID      string `json:"sid"`
		CSRF     string `json:"csrf"`
		Validity int    `json:"validity"`
	} `json:"session"`
}

// configResponse represents the response from /api/config/dns/hosts
type configResponse struct {
	Config struct {
		DNS struct {
			Hosts []string `json:"hosts"`
		} `json:"dns"`
	} `json:"config"`
	Took float64 `json:"took"`
}

// authenticate obtains a session from Pi-hole v6 API
func (c *HTTPClient) authenticate(ctx context.Context) error {
	reqURL := fmt.Sprintf("%s/api/auth", c.baseURL)

	payload := map[string]string{"password": c.password}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshaling auth request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating auth request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("executing auth request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusUnauthorized {
		return &APIError{StatusCode: resp.StatusCode, Message: "invalid password"}
	}
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return &APIError{StatusCode: resp.StatusCode, Message: string(respBody)}
	}

	var authResp authResponse
	if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
		return fmt.Errorf("decoding auth response: %w", err)
	}

	c.mu.Lock()
	c.sid = authResp.Session.SID
	c.csrf = authResp.Session.CSRF
	// Set validity with some buffer (use 80% of the timeout)
	c.valid = time.Now().Add(time.Duration(authResp.Session.Validity*80/100) * time.Second)
	c.mu.Unlock()

	return nil
}

// ensureAuthenticated checks if we have a valid session, authenticates if not
func (c *HTTPClient) ensureAuthenticated(ctx context.Context) error {
	c.mu.RLock()
	valid := c.valid.After(time.Now()) && c.sid != ""
	c.mu.RUnlock()

	if valid {
		return nil
	}

	return c.authenticate(ctx)
}

// setAuthHeaders adds authentication headers to a request
func (c *HTTPClient) setAuthHeaders(req *http.Request) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	req.Header.Set("X-FTL-SID", c.sid)
}

// ListRecords fetches all local DNS records from Pi-hole
func (c *HTTPClient) ListRecords(ctx context.Context) ([]DNSRecord, error) {
	if err := c.ensureAuthenticated(ctx); err != nil {
		return nil, fmt.Errorf("authentication failed: %w", err)
	}

	reqURL := fmt.Sprintf("%s/api/config/dns/hosts", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	c.setAuthHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusUnauthorized {
		// Session expired, try to re-authenticate once
		if err := c.authenticate(ctx); err != nil {
			return nil, fmt.Errorf("re-authentication failed: %w", err)
		}
		return c.ListRecords(ctx)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, &APIError{StatusCode: resp.StatusCode, Message: string(body)}
	}

	var configResp configResponse
	if err := json.NewDecoder(resp.Body).Decode(&configResp); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	// Parse the hosts array (format: "IP DOMAIN")
	var records []DNSRecord
	for _, entry := range configResp.Config.DNS.Hosts {
		parts := strings.Fields(entry)
		if len(parts) >= 2 {
			records = append(records, DNSRecord{
				IP:     parts[0],
				Domain: parts[1],
			})
		}
	}

	return records, nil
}

// CreateRecord creates a new DNS A record in Pi-hole
func (c *HTTPClient) CreateRecord(ctx context.Context, record DNSRecord) error {
	if err := c.ensureAuthenticated(ctx); err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	// Format: "IP DOMAIN" URL-encoded
	entry := fmt.Sprintf("%s %s", record.IP, record.Domain)
	reqURL := fmt.Sprintf("%s/api/config/dns/hosts/%s", c.baseURL, url.PathEscape(entry))

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, reqURL, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	c.setAuthHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("executing request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusUnauthorized {
		// Session expired, try to re-authenticate once
		if err := c.authenticate(ctx); err != nil {
			return fmt.Errorf("re-authentication failed: %w", err)
		}
		return c.CreateRecord(ctx, record)
	}

	// Accept 200, 201, 204 as success
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusNoContent {
		respBody, _ := io.ReadAll(resp.Body)
		return &APIError{StatusCode: resp.StatusCode, Message: string(respBody)}
	}

	return nil
}

// DeleteRecord deletes a DNS record from Pi-hole
func (c *HTTPClient) DeleteRecord(ctx context.Context, domain string) error {
	if err := c.ensureAuthenticated(ctx); err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	// First, we need to find the record to get the full "IP DOMAIN" entry
	records, err := c.ListRecords(ctx)
	if err != nil {
		return fmt.Errorf("listing records to find entry: %w", err)
	}

	var entryToDelete string
	for _, r := range records {
		if r.Domain == domain {
			entryToDelete = fmt.Sprintf("%s %s", r.IP, r.Domain)
			break
		}
	}

	if entryToDelete == "" {
		// Record doesn't exist, consider it a success
		return nil
	}

	reqURL := fmt.Sprintf("%s/api/config/dns/hosts/%s", c.baseURL, url.PathEscape(entryToDelete))

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, reqURL, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	c.setAuthHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("executing request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusUnauthorized {
		// Session expired, try to re-authenticate once
		if err := c.authenticate(ctx); err != nil {
			return fmt.Errorf("re-authentication failed: %w", err)
		}
		return c.DeleteRecord(ctx, domain)
	}

	// Accept 200, 204, 404 as success
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotFound {
		respBody, _ := io.ReadAll(resp.Body)
		return &APIError{StatusCode: resp.StatusCode, Message: string(respBody)}
	}

	return nil
}

// Healthy checks if the Pi-hole API is reachable and authentication works
func (c *HTTPClient) Healthy(ctx context.Context) bool {
	if err := c.ensureAuthenticated(ctx); err != nil {
		return false
	}
	_, err := c.ListRecords(ctx)
	return err == nil
}

// APIError represents an error from the Pi-hole API
type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("pihole api error (status %d): %s", e.StatusCode, e.Message)
}

// IsRetryable returns true if the error should trigger a retry
func (e *APIError) IsRetryable() bool {
	// 400 Bad Request should not retry (invalid request)
	// 401/403 auth issues, 5xx server errors, and other errors should retry
	return e.StatusCode != http.StatusBadRequest
}
