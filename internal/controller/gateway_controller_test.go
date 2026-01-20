package controller

import (
	"log/slog"
	"os"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
)

func TestHTTPRouteExtractHosts(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	r := &HTTPRouteReconciler{Logger: logger}

	tests := []struct {
		name        string
		annotations map[string]string
		hostnames   []gatewayv1.Hostname
		want        []string
	}{
		{
			name:      "extract from spec.hostnames",
			hostnames: []gatewayv1.Hostname{"app.local", "api.local"},
			want:      []string{"app.local", "api.local"},
		},
		{
			name:      "skip empty hostnames",
			hostnames: []gatewayv1.Hostname{"app.local", "", "api.local"},
			want:      []string{"app.local", "api.local"},
		},
		{
			name:      "no hostnames",
			hostnames: []gatewayv1.Hostname{},
			want:      nil,
		},
		{
			name: "override with annotation",
			annotations: map[string]string{
				AnnotationHosts: "custom.local,override.local",
			},
			hostnames: []gatewayv1.Hostname{"app.local"},
			want:      []string{"custom.local", "override.local"},
		},
		{
			name: "annotation with spaces",
			annotations: map[string]string{
				AnnotationHosts: " host1.local , host2.local ",
			},
			want: []string{"host1.local", "host2.local"},
		},
		{
			name: "empty annotation falls back to hostnames",
			annotations: map[string]string{
				AnnotationHosts: "",
			},
			hostnames: []gatewayv1.Hostname{"app.local"},
			want:      []string{"app.local"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			route := &gatewayv1.HTTPRoute{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: tt.annotations,
				},
				Spec: gatewayv1.HTTPRouteSpec{
					Hostnames: tt.hostnames,
				},
			}
			got := r.extractHosts(route)
			if !slicesEqual(got, tt.want) {
				t.Errorf("extractHosts() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGRPCRouteExtractHosts(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	r := &GRPCRouteReconciler{Logger: logger}

	tests := []struct {
		name        string
		annotations map[string]string
		hostnames   []gatewayv1.Hostname
		want        []string
	}{
		{
			name:      "extract from spec.hostnames",
			hostnames: []gatewayv1.Hostname{"grpc.local", "api.local"},
			want:      []string{"grpc.local", "api.local"},
		},
		{
			name: "override with annotation",
			annotations: map[string]string{
				AnnotationHosts: "custom.local",
			},
			hostnames: []gatewayv1.Hostname{"grpc.local"},
			want:      []string{"custom.local"},
		},
		{
			name:      "no hostnames",
			hostnames: []gatewayv1.Hostname{},
			want:      nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			route := &gatewayv1.GRPCRoute{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: tt.annotations,
				},
				Spec: gatewayv1.GRPCRouteSpec{
					Hostnames: tt.hostnames,
				},
			}
			got := r.extractHosts(route)
			if !slicesEqual(got, tt.want) {
				t.Errorf("extractHosts() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTLSRouteExtractHosts(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	r := &TLSRouteReconciler{Logger: logger}

	tests := []struct {
		name        string
		annotations map[string]string
		hostnames   []gatewayv1alpha2.Hostname
		want        []string
	}{
		{
			name:      "extract from spec.hostnames",
			hostnames: []gatewayv1alpha2.Hostname{"tls.local", "secure.local"},
			want:      []string{"tls.local", "secure.local"},
		},
		{
			name: "override with annotation",
			annotations: map[string]string{
				AnnotationHosts: "custom.local",
			},
			hostnames: []gatewayv1alpha2.Hostname{"tls.local"},
			want:      []string{"custom.local"},
		},
		{
			name:      "no hostnames",
			hostnames: []gatewayv1alpha2.Hostname{},
			want:      nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			route := &gatewayv1alpha2.TLSRoute{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: tt.annotations,
				},
				Spec: gatewayv1alpha2.TLSRouteSpec{
					Hostnames: tt.hostnames,
				},
			}
			got := r.extractHosts(route)
			if !slicesEqual(got, tt.want) {
				t.Errorf("extractHosts() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTCPRouteExtractHosts(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	r := &TCPRouteReconciler{Logger: logger}

	tests := []struct {
		name        string
		annotations map[string]string
		want        []string
	}{
		{
			name:        "no annotations returns nil",
			annotations: nil,
			want:        nil,
		},
		{
			name: "hosts from annotation",
			annotations: map[string]string{
				AnnotationHosts: "tcp.local,stream.local",
			},
			want: []string{"tcp.local", "stream.local"},
		},
		{
			name: "empty annotation returns nil",
			annotations: map[string]string{
				AnnotationHosts: "",
			},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			route := &gatewayv1alpha2.TCPRoute{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: tt.annotations,
				},
			}
			got := r.extractHosts(route)
			if !slicesEqual(got, tt.want) {
				t.Errorf("extractHosts() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHasRegistrationAnnotationGateway(t *testing.T) {
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
			got := hasRegistrationAnnotation(tt.annotations)
			if got != tt.want {
				t.Errorf("hasRegistrationAnnotation() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResolveTargetIPGateway(t *testing.T) {
	defaultIP := "192.168.1.100"

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
			got := resolveTargetIP(tt.annotations, defaultIP)
			if got != tt.want {
				t.Errorf("resolveTargetIP() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetManagedHostsGateway(t *testing.T) {
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
			got := getManagedHosts(tt.annotations)
			if !slicesEqual(got, tt.want) {
				t.Errorf("getManagedHosts() = %v, want %v", got, tt.want)
			}
		})
	}
}
