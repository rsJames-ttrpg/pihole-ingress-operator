package pihole

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const (
	testPassword = "test-password"
	testSID      = "test-session-id"
)

// mockAuthServer creates a test server that handles auth and config endpoints
func mockAuthServer(_ *testing.T, hosts []string, authOK bool) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/auth" && r.Method == http.MethodPost:
			// Handle authentication
			var payload map[string]string
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			if !authOK || payload["password"] != testPassword {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error":{"key":"unauthorized"}}`))
				return
			}
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"session": map[string]any{
					"sid":      testSID,
					"csrf":     "test-csrf",
					"validity": 300,
				},
			})

		case r.URL.Path == "/api/config/dns/hosts" && r.Method == http.MethodGet:
			// Handle list records
			if r.Header.Get("X-FTL-SID") != testSID {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"config": map[string]any{
					"dns": map[string]any{
						"hosts": hosts,
					},
				},
				"took": 0.001,
			})

		case strings.HasPrefix(r.URL.Path, "/api/config/dns/hosts/") && r.Method == http.MethodPut:
			// Handle create record
			if r.Header.Get("X-FTL-SID") != testSID {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.WriteHeader(http.StatusCreated)

		case strings.HasPrefix(r.URL.Path, "/api/config/dns/hosts/") && r.Method == http.MethodDelete:
			// Handle delete record
			if r.Header.Get("X-FTL-SID") != testSID {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.WriteHeader(http.StatusNoContent)

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func TestAuthenticate(t *testing.T) {
	tests := []struct {
		name    string
		authOK  bool
		wantErr bool
	}{
		{
			name:    "successful auth",
			authOK:  true,
			wantErr: false,
		},
		{
			name:    "invalid password",
			authOK:  false,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := mockAuthServer(t, nil, tt.authOK)
			defer server.Close()

			client := NewClient(server.URL, testPassword)
			err := client.authenticate(context.Background())

			if tt.wantErr {
				if err == nil {
					t.Error("authenticate() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("authenticate() unexpected error: %v", err)
				return
			}

			if client.sid != testSID {
				t.Errorf("sid = %q, want %q", client.sid, testSID)
			}
		})
	}
}

func TestListRecords(t *testing.T) {
	tests := []struct {
		name      string
		hosts     []string
		wantCount int
	}{
		{
			name:      "empty list",
			hosts:     []string{},
			wantCount: 0,
		},
		{
			name:      "single record",
			hosts:     []string{"192.168.1.100 app.local"},
			wantCount: 1,
		},
		{
			name:      "multiple records",
			hosts:     []string{"192.168.1.100 app.local", "192.168.1.100 api.local"},
			wantCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := mockAuthServer(t, tt.hosts, true)
			defer server.Close()

			client := NewClient(server.URL, testPassword)
			records, err := client.ListRecords(context.Background())

			if err != nil {
				t.Errorf("ListRecords() unexpected error: %v", err)
				return
			}

			if len(records) != tt.wantCount {
				t.Errorf("ListRecords() returned %d records, want %d", len(records), tt.wantCount)
			}

			// Verify record parsing
			for i, host := range tt.hosts {
				parts := strings.Fields(host)
				if len(parts) >= 2 {
					if records[i].IP != parts[0] {
						t.Errorf("records[%d].IP = %q, want %q", i, records[i].IP, parts[0])
					}
					if records[i].Domain != parts[1] {
						t.Errorf("records[%d].Domain = %q, want %q", i, records[i].Domain, parts[1])
					}
				}
			}
		})
	}
}

func TestCreateRecord(t *testing.T) {
	server := mockAuthServer(t, nil, true)
	defer server.Close()

	client := NewClient(server.URL, testPassword)
	record := DNSRecord{IP: "192.168.1.100", Domain: "app.local"}

	err := client.CreateRecord(context.Background(), record)
	if err != nil {
		t.Errorf("CreateRecord() unexpected error: %v", err)
	}
}

func TestDeleteRecord(t *testing.T) {
	hosts := []string{"192.168.1.100 app.local", "192.168.1.100 api.local"}
	server := mockAuthServer(t, hosts, true)
	defer server.Close()

	client := NewClient(server.URL, testPassword)

	// Delete existing record
	err := client.DeleteRecord(context.Background(), "app.local")
	if err != nil {
		t.Errorf("DeleteRecord() unexpected error: %v", err)
	}

	// Delete non-existing record should succeed (no-op)
	err = client.DeleteRecord(context.Background(), "nonexistent.local")
	if err != nil {
		t.Errorf("DeleteRecord() for non-existent should not error: %v", err)
	}
}

func TestHealthy(t *testing.T) {
	tests := []struct {
		name   string
		authOK bool
		want   bool
	}{
		{
			name:   "healthy",
			authOK: true,
			want:   true,
		},
		{
			name:   "unhealthy - auth fails",
			authOK: false,
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := mockAuthServer(t, []string{}, tt.authOK)
			defer server.Close()

			client := NewClient(server.URL, testPassword)
			got := client.Healthy(context.Background())

			if got != tt.want {
				t.Errorf("Healthy() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAPIError(t *testing.T) {
	err := &APIError{
		StatusCode: 500,
		Message:    "internal error",
	}

	if err.Error() != "pihole api error (status 500): internal error" {
		t.Errorf("APIError.Error() = %q", err.Error())
	}

	if !err.IsRetryable() {
		t.Error("500 should be retryable")
	}

	err400 := &APIError{StatusCode: 400, Message: "bad request"}
	if err400.IsRetryable() {
		t.Error("400 should not be retryable")
	}
}
