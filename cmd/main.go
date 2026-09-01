package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/mmuazam98/service-sentinel/config"
	"github.com/mmuazam98/service-sentinel/pkg/checker"
	"github.com/mmuazam98/service-sentinel/pkg/k8s"
	"github.com/mmuazam98/service-sentinel/pkg/logger"
)

func main() {
	if err := run(); err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
}

func run() error {
	configPath := flag.String("config", envOr("CONFIG_PATH", config.DefaultPath), "Path to the config file")
	interval := flag.Duration("interval", 0, "Interval between health checks (overrides the config file), e.g. 30s")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	if *interval > 0 {
		cfg.Interval = *interval
	}

	var discoverer checker.Discoverer
	if cfg.Kubernetes.Enabled {
		d, err := k8s.InCluster()
		if err != nil {
			return fmt.Errorf("kubernetes discovery is enabled but unavailable: %w", err)
		}
		discoverer = d
		logger.Info(fmt.Sprintf("Kubernetes discovery enabled (probe=%s, namespaces=%s)",
			cfg.Kubernetes.Probe, namespacesLabel(cfg.Kubernetes.Namespaces)))
	}

	describe(cfg, *configPath)

	// Ctrl-C or a Kubernetes SIGTERM cancels the context, which stops the
	// ticker and unblocks any in-flight health check.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	checker.NewServiceChecker(cfg, discoverer).Run(ctx)

	return nil
}

func describe(cfg config.Config, path string) {
	logger.Info(fmt.Sprintf("Loaded %d service(s) from %s (interval=%s, timeout=%s)",
		len(cfg.Services), path, cfg.Interval, cfg.Timeout))
	for _, service := range cfg.Services {
		logger.Info(fmt.Sprintf("  - %s: %s", service.Name, service.URL))
	}
}

func namespacesLabel(namespaces []string) string {
	if len(namespaces) == 0 {
		return "all"
	}
	return fmt.Sprintf("%v", namespaces)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
