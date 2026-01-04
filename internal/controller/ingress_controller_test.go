package controller

import (
	"log/slog"
	"os"
	"testing"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestHasRegistrationAnnotation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	r := &IngressReconciler{Logger: logger}

	tests := []struct {
		name        string
		annotations map[string]string
		want        bool
	}{
		{
			name:        "no annotations",
			annotations: nil,
			want:        false,
		},
		{
			name:        "empty annotations",
			annotations: map[string]string{},
			want:        false,
		},
		{
			name: "annotation set to true",
			annotations: map[string]string{
				AnnotationRegister: "true",
			},
			want: true,
		},
		{
			name: "annotation set to false",
			annotations: map[string]string{
				AnnotationRegister: "false",
			},
			want: false,
		},
		{
			name: "annotation set to other value",
			annotations: map[string]string{
				AnnotationRegister: "yes",
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ingress := &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: tt.annotations,
				},
			}
			got := r.hasRegistrationAnnotation(ingress)
			if got != tt.want {
				t.Errorf("hasRegistrationAnnotation() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractHosts(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	r := &IngressReconciler{Logger: logger}

	tests := []struct {
		name        string
		annotations map[string]string
		rules       []networkingv1.IngressRule
		want        []string
	}{
		{
			name: "extract from spec.rules",
			rules: []networkingv1.IngressRule{
				{Host: "app.local"},
				{Host: "api.local"},
			},
			want: []string{"app.local", "api.local"},
		},
		{
			name: "skip empty hosts",
			rules: []networkingv1.IngressRule{
				{Host: "app.local"},
				{Host: ""},
				{Host: "api.local"},
			},
			want: []string{"app.local", "api.local"},
		},
		{
			name:  "no rules",
			rules: []networkingv1.IngressRule{},
			want:  nil,
		},
		{
			name: "override with annotation",
			annotations: map[string]string{
				AnnotationHosts: "custom.local,override.local",
			},
			rules: []networkingv1.IngressRule{
				{Host: "app.local"},
			},
			want: []string{"custom.local", "override.local"},
		},
		{
			name: "annotation with spaces",
			annotations: map[string]string{
				AnnotationHosts: " host1.local , host2.local ",
			},
			want: []string{"host1.local", "host2.local"},
		},
		{
			name: "empty annotation falls back to rules",
			annotations: map[string]string{
				AnnotationHosts: "",
			},
			rules: []networkingv1.IngressRule{
				{Host: "app.local"},
			},
			want: []string{"app.local"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ingress := &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: tt.annotations,
				},
				Spec: networkingv1.IngressSpec{
					Rules: tt.rules,
				},
			}
			got := r.extractHosts(ingress)
			if !slicesEqual(got, tt.want) {
				t.Errorf("extractHosts() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResolveTargetIP(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	r := &IngressReconciler{
		Logger:          logger,
		DefaultTargetIP: "192.168.1.100",
	}

	tests := []struct {
		name        string
		annotations map[string]string
		want        string
	}{
		{
			name:        "use default",
			annotations: nil,
			want:        "192.168.1.100",
		},
		{
			name: "override with valid IP",
			annotations: map[string]string{
				AnnotationTargetIP: "10.0.0.1",
			},
			want: "10.0.0.1",
		},
		{
			name: "invalid IP returns empty",
			annotations: map[string]string{
				AnnotationTargetIP: "not-an-ip",
			},
			want: "",
		},
		{
			name: "IPv6 returns empty",
			annotations: map[string]string{
				AnnotationTargetIP: "::1",
			},
			want: "",
		},
		{
			name: "empty annotation uses default",
			annotations: map[string]string{
				AnnotationTargetIP: "",
			},
			want: "192.168.1.100",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ingress := &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: tt.annotations,
				},
			}
			got := r.resolveTargetIP(ingress)
			if got != tt.want {
				t.Errorf("resolveTargetIP() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetManagedHosts(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	r := &IngressReconciler{Logger: logger}

	tests := []struct {
		name        string
		annotations map[string]string
		want        []string
	}{
		{
			name:        "no annotations",
			annotations: nil,
			want:        nil,
		},
		{
			name: "single host",
			annotations: map[string]string{
				AnnotationManagedHosts: "app.local",
			},
			want: []string{"app.local"},
		},
		{
			name: "multiple hosts",
			annotations: map[string]string{
				AnnotationManagedHosts: "app.local,api.local,web.local",
			},
			want: []string{"app.local", "api.local", "web.local"},
		},
		{
			name: "empty annotation",
			annotations: map[string]string{
				AnnotationManagedHosts: "",
			},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ingress := &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: tt.annotations,
				},
			}
			got := r.getManagedHosts(ingress)
			if !slicesEqual(got, tt.want) {
				t.Errorf("getManagedHosts() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseCommaSeparated(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"", []string{}},
		{"single", []string{"single"}},
		{"a,b,c", []string{"a", "b", "c"}},
		{" a , b , c ", []string{"a", "b", "c"}},
		{"a,,b", []string{"a", "b"}},
		{",a,b,", []string{"a", "b"}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseCommaSeparated(tt.input)
			if !slicesEqual(got, tt.want) {
				t.Errorf("parseCommaSeparated(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
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

// slicesEqual compares two string slices for equality
func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
