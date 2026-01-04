/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"time"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/rsJames-ttrpg/pihole-ingress-operator/internal/config"
	"github.com/rsJames-ttrpg/pihole-ingress-operator/internal/controller"
	"github.com/rsJames-ttrpg/pihole-ingress-operator/internal/pihole"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
}

func main() {
	var metricsAddr string
	var probeAddr string
	var enableLeaderElection bool

	flag.StringVar(&metricsAddr, "metrics-bind-address", "0",
		"The address the metrics endpoint binds to. Use :8080 for HTTP, or 0 to disable.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false, "Enable leader election for controller manager.")
	flag.Parse()

	// Load operator configuration
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	// Set up structured logging
	logLevel := slog.LevelInfo
	switch cfg.LogLevel {
	case "debug":
		logLevel = slog.LevelDebug
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))
	slog.SetDefault(logger)

	// Set up controller-runtime logger to use slog
	ctrl.SetLogger(NewSlogLogr(logger))

	// Create Pi-hole client
	piholeClient := pihole.NewClient(cfg.PiholeURL, cfg.PiholePassword)

	// Check Pi-hole connectivity
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	if !piholeClient.Healthy(ctx) {
		logger.Warn("pi-hole is not reachable at startup, will retry during reconciliation", "url", cfg.PiholeURL)
	}
	cancel()

	// Configure manager options
	mgrOpts := ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: metricsAddr,
		},
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "d159a95c.pihole.io",
	}

	// Configure namespace watching
	if cfg.WatchNamespace != "" {
		mgrOpts.Cache = cache.Options{
			DefaultNamespaces: map[string]cache.Config{
				cfg.WatchNamespace: {},
			},
		}
		logger.Info("watching namespace", "namespace", cfg.WatchNamespace)
	} else {
		logger.Info("watching all namespaces")
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), mgrOpts)
	if err != nil {
		logger.Error("unable to start manager", "error", err)
		os.Exit(1)
	}

	// Set up the Ingress controller
	if err := (&controller.IngressReconciler{
		Client:          mgr.GetClient(),
		Scheme:          mgr.GetScheme(),
		PiholeClient:    piholeClient,
		DefaultTargetIP: cfg.DefaultTargetIP,
		Logger:          logger,
	}).SetupWithManager(mgr); err != nil {
		logger.Error("unable to create controller", "controller", "Ingress", "error", err)
		os.Exit(1)
	}

	// Set up health checks
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		logger.Error("unable to set up health check", "error", err)
		os.Exit(1)
	}

	// Readiness check includes Pi-hole connectivity
	if err := mgr.AddReadyzCheck("readyz", healthz.Checker(func(_ *http.Request) error {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if !piholeClient.Healthy(ctx) {
			return errPiholeUnhealthy
		}
		return nil
	})); err != nil {
		logger.Error("unable to set up ready check", "error", err)
		os.Exit(1)
	}

	logger.Info("starting manager", "pihole_url", cfg.PiholeURL, "default_target_ip", cfg.DefaultTargetIP)
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		logger.Error("problem running manager", "error", err)
		os.Exit(1)
	}
}

// errPiholeUnhealthy is returned when Pi-hole is not reachable
var errPiholeUnhealthy = &piholeUnhealthyError{}

type piholeUnhealthyError struct{}

func (e *piholeUnhealthyError) Error() string {
	return "pi-hole is not reachable"
}
