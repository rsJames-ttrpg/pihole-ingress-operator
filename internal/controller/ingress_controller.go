package controller

import (
	"context"
	"log/slog"
	"strings"
	"time"

	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/rsJames-ttrpg/pihole-ingress-operator/internal/pihole"
)

// Annotation keys and FinalizerName are defined in common.go

// IngressReconciler reconciles Ingress objects
type IngressReconciler struct {
	client.Client
	Scheme          *runtime.Scheme
	PiholeClient    pihole.Client
	DefaultTargetIP string
	Logger          *slog.Logger
}

// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Reconcile handles Ingress create/update/delete events
func (r *IngressReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.Logger.With("ingress", req.String())
	logger.Debug("reconcile started")

	var ingress networkingv1.Ingress
	if err := r.Get(ctx, req.NamespacedName, &ingress); err != nil {
		if errors.IsNotFound(err) {
			logger.Debug("ingress not found, likely deleted")
			return ctrl.Result{}, nil
		}
		logger.Error("failed to get ingress", "error", err)
		return ctrl.Result{}, err
	}

	// Check if the ingress is being deleted
	if !ingress.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, &ingress, logger)
	}

	// Check if registration is enabled
	if !r.hasRegistrationAnnotation(&ingress) {
		// Annotation not present or removed - clean up if we have a finalizer
		if controllerutil.ContainsFinalizer(&ingress, FinalizerName) {
			return r.handleDeletion(ctx, &ingress, logger)
		}
		return ctrl.Result{}, nil
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(&ingress, FinalizerName) {
		logger.Debug("adding finalizer")
		controllerutil.AddFinalizer(&ingress, FinalizerName)
		if err := r.Update(ctx, &ingress); err != nil {
			logger.Error("failed to add finalizer", "error", err)
			return ctrl.Result{}, err
		}
		// Re-fetch after update to get the latest version
		if err := r.Get(ctx, req.NamespacedName, &ingress); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Get desired state
	desiredHosts := r.extractHosts(&ingress)
	if len(desiredHosts) == 0 {
		logger.Warn("ingress skipped (no hosts)")
		return ctrl.Result{}, nil
	}

	targetIP := r.resolveTargetIP(&ingress)
	if targetIP == "" {
		logger.Warn("invalid annotation", "annotation", AnnotationTargetIP,
			"value", ingress.Annotations[AnnotationTargetIP], "error", "not a valid IPv4 address")
		return ctrl.Result{}, nil // Don't requeue - user needs to fix annotation
	}

	// Get current Pi-hole records
	currentRecords, err := r.PiholeClient.ListRecords(ctx)
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
	managedHosts := r.getManagedHosts(&ingress)

	// Sync desired records
	for _, host := range desiredHosts {
		currentIP, exists := currentRecordMap[host]
		if !exists || currentIP != targetIP {
			// Need to create or update
			if exists && currentIP != targetIP {
				// Delete old record first (Pi-hole doesn't support update)
				if err := r.PiholeClient.DeleteRecord(ctx, host); err != nil {
					logger.Error("pihole api error", "operation", "delete", "error", err)
					return r.handleAPIError(err, logger)
				}
				logger.Info("dns record updated", "host", host, "old_ip", currentIP, "new_ip", targetIP)
			}

			record := pihole.DNSRecord{Domain: host, IP: targetIP}
			if err := r.PiholeClient.CreateRecord(ctx, record); err != nil {
				logger.Error("pihole api error", "operation", "create", "error", err)
				return r.handleAPIError(err, logger)
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
			if err := r.PiholeClient.DeleteRecord(ctx, host); err != nil {
				logger.Error("pihole api error", "operation", "delete", "error", err)
				return r.handleAPIError(err, logger)
			}
			logger.Info("dns record deleted", "host", host)
		}
	}

	// Update managed hosts annotation
	if err := r.updateManagedHosts(ctx, &ingress, desiredHosts); err != nil {
		logger.Error("failed to update managed hosts annotation", "error", err)
		return ctrl.Result{RequeueAfter: 10 * time.Second}, err
	}

	return ctrl.Result{}, nil
}

// handleDeletion cleans up DNS records and removes finalizer
func (r *IngressReconciler) handleDeletion(ctx context.Context, ingress *networkingv1.Ingress, logger *slog.Logger) (ctrl.Result, error) {
	// Check if we have our finalizer
	if !controllerutil.ContainsFinalizer(ingress, FinalizerName) {
		return ctrl.Result{}, nil
	}

	// Clean up DNS records
	managedHosts := r.getManagedHosts(ingress)
	for _, host := range managedHosts {
		if err := r.PiholeClient.DeleteRecord(ctx, host); err != nil {
			logger.Error("pihole api error", "operation", "delete", "error", err)
			return r.handleAPIError(err, logger)
		}
		logger.Info("dns record deleted", "host", host)
	}

	// Remove finalizer
	logger.Debug("removing finalizer")
	controllerutil.RemoveFinalizer(ingress, FinalizerName)
	if err := r.Update(ctx, ingress); err != nil {
		logger.Error("failed to remove finalizer", "error", err)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// handleAPIError determines the requeue behavior based on the error type
func (r *IngressReconciler) handleAPIError(err error, logger *slog.Logger) (ctrl.Result, error) {
	if apiErr, ok := err.(*pihole.APIError); ok {
		if !apiErr.IsRetryable() {
			logger.Warn("non-retryable api error", "error", err)
			return ctrl.Result{}, nil // Don't requeue
		}
	}
	return ctrl.Result{RequeueAfter: 30 * time.Second}, err
}

// hasRegistrationAnnotation checks if the Ingress has the registration annotation set to "true"
func (r *IngressReconciler) hasRegistrationAnnotation(ingress *networkingv1.Ingress) bool {
	if ingress.Annotations == nil {
		return false
	}
	return ingress.Annotations[AnnotationRegister] == "true"
}

// extractHosts gets the list of hostnames from the Ingress
func (r *IngressReconciler) extractHosts(ingress *networkingv1.Ingress) []string {
	// Check for override annotation
	if ingress.Annotations != nil {
		if hostsAnnotation, ok := ingress.Annotations[AnnotationHosts]; ok && hostsAnnotation != "" {
			return parseCommaSeparated(hostsAnnotation)
		}
	}

	// Extract from spec.rules
	var hosts []string
	for _, rule := range ingress.Spec.Rules {
		if rule.Host != "" {
			hosts = append(hosts, rule.Host)
		}
	}
	return hosts
}

// resolveTargetIP determines the target IP for DNS records
func (r *IngressReconciler) resolveTargetIP(ingress *networkingv1.Ingress) string {
	// Check for per-Ingress override
	if ingress.Annotations != nil {
		if ip, ok := ingress.Annotations[AnnotationTargetIP]; ok && ip != "" {
			if isValidIPv4(ip) {
				return ip
			}
			return "" // Invalid IP - return empty to signal error
		}
	}
	return r.DefaultTargetIP
}

// getManagedHosts returns the list of hosts currently managed for this Ingress
func (r *IngressReconciler) getManagedHosts(ingress *networkingv1.Ingress) []string {
	if ingress.Annotations == nil {
		return nil
	}
	managed, ok := ingress.Annotations[AnnotationManagedHosts]
	if !ok || managed == "" {
		return nil
	}
	return parseCommaSeparated(managed)
}

// updateManagedHosts updates the managed-hosts annotation on the Ingress
func (r *IngressReconciler) updateManagedHosts(ctx context.Context, ingress *networkingv1.Ingress, hosts []string) error {
	// Get fresh copy to avoid conflicts
	var fresh networkingv1.Ingress
	if err := r.Get(ctx, client.ObjectKeyFromObject(ingress), &fresh); err != nil {
		return err
	}

	if fresh.Annotations == nil {
		fresh.Annotations = make(map[string]string)
	}

	if len(hosts) == 0 {
		delete(fresh.Annotations, AnnotationManagedHosts)
	} else {
		fresh.Annotations[AnnotationManagedHosts] = strings.Join(hosts, ",")
	}

	return r.Update(ctx, &fresh)
}

// SetupWithManager sets up the controller with the Manager
func (r *IngressReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&networkingv1.Ingress{}).
		Named("ingress").
		Complete(r)
}
