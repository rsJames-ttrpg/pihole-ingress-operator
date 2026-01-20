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
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"

	"github.com/rsJames-ttrpg/pihole-ingress-operator/internal/config"
	"github.com/rsJames-ttrpg/pihole-ingress-operator/internal/controller"
	"github.com/rsJames-ttrpg/pihole-ingress-operator/internal/pihole"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(gatewayv1.Install(scheme))
	utilruntime.Must(gatewayv1alpha2.Install(scheme))
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

	// Set up Gateway API controllers
	if err := (&controller.HTTPRouteReconciler{
		Client:          mgr.GetClient(),
		Scheme:          mgr.GetScheme(),
		PiholeClient:    piholeClient,
		DefaultTargetIP: cfg.DefaultTargetIP,
		Logger:          logger,
	}).SetupWithManager(mgr); err != nil {
		logger.Error("unable to create controller", "controller", "HTTPRoute", "error", err)
		os.Exit(1)
	}

	if err := (&controller.GRPCRouteReconciler{
		Client:          mgr.GetClient(),
		Scheme:          mgr.GetScheme(),
		PiholeClient:    piholeClient,
		DefaultTargetIP: cfg.DefaultTargetIP,
		Logger:          logger,
	}).SetupWithManager(mgr); err != nil {
		logger.Error("unable to create controller", "controller", "GRPCRoute", "error", err)
		os.Exit(1)
	}

	if err := (&controller.TLSRouteReconciler{
		Client:          mgr.GetClient(),
		Scheme:          mgr.GetScheme(),
		PiholeClient:    piholeClient,
		DefaultTargetIP: cfg.DefaultTargetIP,
		Logger:          logger,
	}).SetupWithManager(mgr); err != nil {
		logger.Error("unable to create controller", "controller", "TLSRoute", "error", err)
		os.Exit(1)
	}

	if err := (&controller.TCPRouteReconciler{
		Client:          mgr.GetClient(),
		Scheme:          mgr.GetScheme(),
		PiholeClient:    piholeClient,
		DefaultTargetIP: cfg.DefaultTargetIP,
		Logger:          logger,
	}).SetupWithManager(mgr); err != nil {
		logger.Error("unable to create controller", "controller", "TCPRoute", "error", err)
		os.Exit(1)
	}

	// Set up health checks
	// Both liveness and readiness use simple ping - the operator can function
	// even if Pi-hole is temporarily unavailable (it will retry during reconciliation)
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		logger.Error("unable to set up health check", "error", err)
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		logger.Error("unable to set up ready check", "error", err)
		os.Exit(1)
	}

	logger.Info("starting manager", "pihole_url", cfg.PiholeURL, "default_target_ip", cfg.DefaultTargetIP)
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		logger.Error("problem running manager", "error", err)
		os.Exit(1)
	}
}
