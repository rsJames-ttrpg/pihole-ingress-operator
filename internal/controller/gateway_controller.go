package controller

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"

	"github.com/rsJames-ttrpg/pihole-ingress-operator/internal/pihole"
)

// HTTPRouteReconciler reconciles HTTPRoute objects
type HTTPRouteReconciler struct {
	client.Client
	Scheme          *runtime.Scheme
	PiholeClient    pihole.Client
	DefaultTargetIP string
	Logger          *slog.Logger
}

// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=httproutes,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=httproutes/finalizers,verbs=update

// Reconcile handles HTTPRoute create/update/delete events
func (r *HTTPRouteReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.Logger.With("httproute", req.String())
	logger.Debug("reconcile started")

	var route gatewayv1.HTTPRoute
	if err := r.Get(ctx, req.NamespacedName, &route); err != nil {
		if errors.IsNotFound(err) {
			logger.Debug("httproute not found, likely deleted")
			return ctrl.Result{}, nil
		}
		logger.Error("failed to get httproute", "error", err)
		return ctrl.Result{}, err
	}

	return reconcileRoute(ctx, r.Client, r.PiholeClient, r.DefaultTargetIP, logger,
		&route, route.Annotations, r.extractHosts(&route))
}

func (r *HTTPRouteReconciler) extractHosts(route *gatewayv1.HTTPRoute) []string {
	// Check for override annotation
	if route.Annotations != nil {
		if hostsAnnotation, ok := route.Annotations[AnnotationHosts]; ok && hostsAnnotation != "" {
			return parseCommaSeparated(hostsAnnotation)
		}
	}

	// Extract from spec.hostnames
	var hosts []string
	for _, hostname := range route.Spec.Hostnames {
		if hostname != "" {
			hosts = append(hosts, string(hostname))
		}
	}
	return hosts
}

// SetupWithManager sets up the controller with the Manager
func (r *HTTPRouteReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&gatewayv1.HTTPRoute{}).
		Named("httproute").
		Complete(r)
}

// GRPCRouteReconciler reconciles GRPCRoute objects
type GRPCRouteReconciler struct {
	client.Client
	Scheme          *runtime.Scheme
	PiholeClient    pihole.Client
	DefaultTargetIP string
	Logger          *slog.Logger
}

// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=grpcroutes,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=grpcroutes/finalizers,verbs=update

// Reconcile handles GRPCRoute create/update/delete events
func (r *GRPCRouteReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.Logger.With("grpcroute", req.String())
	logger.Debug("reconcile started")

	var route gatewayv1.GRPCRoute
	if err := r.Get(ctx, req.NamespacedName, &route); err != nil {
		if errors.IsNotFound(err) {
			logger.Debug("grpcroute not found, likely deleted")
			return ctrl.Result{}, nil
		}
		logger.Error("failed to get grpcroute", "error", err)
		return ctrl.Result{}, err
	}

	return reconcileRoute(ctx, r.Client, r.PiholeClient, r.DefaultTargetIP, logger,
		&route, route.Annotations, r.extractHosts(&route))
}

func (r *GRPCRouteReconciler) extractHosts(route *gatewayv1.GRPCRoute) []string {
	// Check for override annotation
	if route.Annotations != nil {
		if hostsAnnotation, ok := route.Annotations[AnnotationHosts]; ok && hostsAnnotation != "" {
			return parseCommaSeparated(hostsAnnotation)
		}
	}

	// Extract from spec.hostnames
	var hosts []string
	for _, hostname := range route.Spec.Hostnames {
		if hostname != "" {
			hosts = append(hosts, string(hostname))
		}
	}
	return hosts
}

// SetupWithManager sets up the controller with the Manager
func (r *GRPCRouteReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&gatewayv1.GRPCRoute{}).
		Named("grpcroute").
		Complete(r)
}

// TLSRouteReconciler reconciles TLSRoute objects
type TLSRouteReconciler struct {
	client.Client
	Scheme          *runtime.Scheme
	PiholeClient    pihole.Client
	DefaultTargetIP string
	Logger          *slog.Logger
}

// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=tlsroutes,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=tlsroutes/finalizers,verbs=update

// Reconcile handles TLSRoute create/update/delete events
func (r *TLSRouteReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.Logger.With("tlsroute", req.String())
	logger.Debug("reconcile started")

	var route gatewayv1alpha2.TLSRoute
	if err := r.Get(ctx, req.NamespacedName, &route); err != nil {
		if errors.IsNotFound(err) {
			logger.Debug("tlsroute not found, likely deleted")
			return ctrl.Result{}, nil
		}
		logger.Error("failed to get tlsroute", "error", err)
		return ctrl.Result{}, err
	}

	return reconcileRoute(ctx, r.Client, r.PiholeClient, r.DefaultTargetIP, logger,
		&route, route.Annotations, r.extractHosts(&route))
}

func (r *TLSRouteReconciler) extractHosts(route *gatewayv1alpha2.TLSRoute) []string {
	// Check for override annotation
	if route.Annotations != nil {
		if hostsAnnotation, ok := route.Annotations[AnnotationHosts]; ok && hostsAnnotation != "" {
			return parseCommaSeparated(hostsAnnotation)
		}
	}

	// Extract from spec.hostnames
	var hosts []string
	for _, hostname := range route.Spec.Hostnames {
		if hostname != "" {
			hosts = append(hosts, string(hostname))
		}
	}
	return hosts
}

// SetupWithManager sets up the controller with the Manager
func (r *TLSRouteReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&gatewayv1alpha2.TLSRoute{}).
		Named("tlsroute").
		Complete(r)
}

// TCPRouteReconciler reconciles TCPRoute objects
type TCPRouteReconciler struct {
	client.Client
	Scheme          *runtime.Scheme
	PiholeClient    pihole.Client
	DefaultTargetIP string
	Logger          *slog.Logger
}

// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=tcproutes,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=tcproutes/finalizers,verbs=update

// Reconcile handles TCPRoute create/update/delete events
func (r *TCPRouteReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.Logger.With("tcproute", req.String())
	logger.Debug("reconcile started")

	var route gatewayv1alpha2.TCPRoute
	if err := r.Get(ctx, req.NamespacedName, &route); err != nil {
		if errors.IsNotFound(err) {
			logger.Debug("tcproute not found, likely deleted")
			return ctrl.Result{}, nil
		}
		logger.Error("failed to get tcproute", "error", err)
		return ctrl.Result{}, err
	}

	return reconcileRoute(ctx, r.Client, r.PiholeClient, r.DefaultTargetIP, logger,
		&route, route.Annotations, r.extractHosts(&route))
}

func (r *TCPRouteReconciler) extractHosts(route *gatewayv1alpha2.TCPRoute) []string {
	// TCPRoute doesn't have spec.hostnames - only annotation override
	if route.Annotations != nil {
		if hostsAnnotation, ok := route.Annotations[AnnotationHosts]; ok && hostsAnnotation != "" {
			return parseCommaSeparated(hostsAnnotation)
		}
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager
func (r *TCPRouteReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&gatewayv1alpha2.TCPRoute{}).
		Named("tcproute").
		Complete(r)
}

// reconcileRoute is the shared reconciliation logic for all Gateway API route types
func reconcileRoute(
	ctx context.Context,
	k8sClient client.Client,
	piholeClient pihole.Client,
	defaultTargetIP string,
	logger *slog.Logger,
	obj client.Object,
	annotations map[string]string,
	extractedHosts []string,
) (ctrl.Result, error) {
	// Check if the route is being deleted
	if !obj.GetDeletionTimestamp().IsZero() {
		return handleRouteDeletion(ctx, k8sClient, piholeClient, logger, obj, annotations)
	}

	// Check if registration is enabled
	if !hasRegistrationAnnotation(annotations) {
		// Annotation not present or removed - clean up if we have a finalizer
		if controllerutil.ContainsFinalizer(obj, FinalizerName) {
			return handleRouteDeletion(ctx, k8sClient, piholeClient, logger, obj, annotations)
		}
		return ctrl.Result{}, nil
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(obj, FinalizerName) {
		logger.Debug("adding finalizer")
		controllerutil.AddFinalizer(obj, FinalizerName)
		if err := k8sClient.Update(ctx, obj); err != nil {
			logger.Error("failed to add finalizer", "error", err)
			return ctrl.Result{}, err
		}
		// Re-fetch after update to get the latest version
		if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(obj), obj); err != nil {
			return ctrl.Result{}, err
		}
		// Update annotations reference after re-fetch
		annotations = obj.GetAnnotations()
	}

	// Get desired state
	desiredHosts := extractedHosts
	if len(desiredHosts) == 0 {
		logger.Warn("route skipped (no hosts)")
		return ctrl.Result{}, nil
	}

	targetIP := resolveTargetIP(annotations, defaultTargetIP)
	if targetIP == "" {
		logger.Warn("invalid annotation", "annotation", AnnotationTargetIP,
			"value", annotations[AnnotationTargetIP], "error", "not a valid IPv4 address")
		return ctrl.Result{}, nil // Don't requeue - user needs to fix annotation
	}

	// Get current Pi-hole records
	currentRecords, err := piholeClient.ListRecords(ctx)
	if err != nil {
		logger.Error("pihole api error", "operation", "list", "error", err)
		return ctrl.Result{RequeueAfter: 30 * time.Second}, err
	}

	// Build a map of current records for quick lookup
	currentRecordMap := make(map[string]string)
	for _, record := range currentRecords {
		currentRecordMap[record.Domain] = record.IP
	}

	// Get previously managed hosts
	managedHosts := getManagedHosts(annotations)

	// Sync desired records
	for _, host := range desiredHosts {
		currentIP, exists := currentRecordMap[host]
		if !exists || currentIP != targetIP {
			// Need to create or update
			if exists && currentIP != targetIP {
				// Delete old record first (Pi-hole doesn't support update)
				if err := piholeClient.DeleteRecord(ctx, host); err != nil {
					logger.Error("pihole api error", "operation", "delete", "error", err)
					return handleAPIError(err, logger)
				}
				logger.Info("dns record updated", "host", host, "old_ip", currentIP, "new_ip", targetIP)
			}

			record := pihole.DNSRecord{Domain: host, IP: targetIP}
			if err := piholeClient.CreateRecord(ctx, record); err != nil {
				logger.Error("pihole api error", "operation", "create", "error", err)
				return handleAPIError(err, logger)
			}

			if !exists {
				logger.Info("dns record created", "host", host, "ip", targetIP)
			}
		}
	}

	// Delete records for hosts no longer desired
	desiredHostSet := make(map[string]bool)
	for _, h := range desiredHosts {
		desiredHostSet[h] = true
	}

	for _, host := range managedHosts {
		if !desiredHostSet[host] {
			if err := piholeClient.DeleteRecord(ctx, host); err != nil {
				logger.Error("pihole api error", "operation", "delete", "error", err)
				return handleAPIError(err, logger)
			}
			logger.Info("dns record deleted", "host", host)
		}
	}

	// Update managed hosts annotation
	if err := updateManagedHosts(ctx, k8sClient, obj, desiredHosts); err != nil {
		logger.Error("failed to update managed hosts annotation", "error", err)
		return ctrl.Result{RequeueAfter: 10 * time.Second}, err
	}

	return ctrl.Result{}, nil
}

// handleRouteDeletion cleans up DNS records and removes finalizer for Gateway routes
func handleRouteDeletion(
	ctx context.Context,
	k8sClient client.Client,
	piholeClient pihole.Client,
	logger *slog.Logger,
	obj client.Object,
	annotations map[string]string,
) (ctrl.Result, error) {
	// Check if we have our finalizer
	if !controllerutil.ContainsFinalizer(obj, FinalizerName) {
		return ctrl.Result{}, nil
	}

	// Clean up DNS records
	managedHosts := getManagedHosts(annotations)
	for _, host := range managedHosts {
		if err := piholeClient.DeleteRecord(ctx, host); err != nil {
			logger.Error("pihole api error", "operation", "delete", "error", err)
			return handleAPIError(err, logger)
		}
		logger.Info("dns record deleted", "host", host)
	}

	// Remove finalizer
	logger.Debug("removing finalizer")
	controllerutil.RemoveFinalizer(obj, FinalizerName)
	if err := k8sClient.Update(ctx, obj); err != nil {
		logger.Error("failed to remove finalizer", "error", err)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// hasRegistrationAnnotation checks if the route has the registration annotation set to "true"
func hasRegistrationAnnotation(annotations map[string]string) bool {
	if annotations == nil {
		return false
	}
	return annotations[AnnotationRegister] == "true"
}

// resolveTargetIP determines the target IP for DNS records
func resolveTargetIP(annotations map[string]string, defaultTargetIP string) string {
	// Check for per-route override
	if annotations != nil {
		if ip, ok := annotations[AnnotationTargetIP]; ok && ip != "" {
			if isValidIPv4(ip) {
				return ip
			}
			return "" // Invalid IP - return empty to signal error
		}
	}
	return defaultTargetIP
}

// getManagedHosts returns the list of hosts currently managed for this route
func getManagedHosts(annotations map[string]string) []string {
	if annotations == nil {
		return nil
	}
	managed, ok := annotations[AnnotationManagedHosts]
	if !ok || managed == "" {
		return nil
	}
	return parseCommaSeparated(managed)
}

// updateManagedHosts updates the managed-hosts annotation on the route
func updateManagedHosts(ctx context.Context, k8sClient client.Client, obj client.Object, hosts []string) error {
	// Get fresh copy to avoid conflicts
	freshObj := obj.DeepCopyObject().(client.Object)
	if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(obj), freshObj); err != nil {
		return err
	}

	annotations := freshObj.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}

	if len(hosts) == 0 {
		delete(annotations, AnnotationManagedHosts)
	} else {
		annotations[AnnotationManagedHosts] = strings.Join(hosts, ",")
	}

	freshObj.SetAnnotations(annotations)
	return k8sClient.Update(ctx, freshObj)
}

// handleAPIError determines the requeue behavior based on the error type
func handleAPIError(err error, logger *slog.Logger) (ctrl.Result, error) {
	if apiErr, ok := err.(*pihole.APIError); ok {
		if !apiErr.IsRetryable() {
			logger.Warn("non-retryable api error", "error", err)
			return ctrl.Result{}, nil // Don't requeue
		}
	}
	return ctrl.Result{RequeueAfter: 30 * time.Second}, err
}
