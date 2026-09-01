package checker

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/mmuazam98/service-sentinel/config"
	"github.com/mmuazam98/service-sentinel/pkg/alert"
	"github.com/mmuazam98/service-sentinel/pkg/logger"
)

// Discoverer finds services to check at runtime. It is nil when Kubernetes
// discovery is disabled.
type Discoverer interface {
	Discover(ctx context.Context, cfg config.Kubernetes) ([]config.Service, error)
}

type ServiceChecker struct {
	Config     config.Config
	Discoverer Discoverer

	// client verifies TLS normally and is used for services from config.
	client *http.Client
	// probeClient skips TLS verification and is used ONLY for endpoints
	// discovered in-cluster. The kubelet does not verify HTTPS probes
	// either: a probe targets a pod IP, and pod certificates cannot carry
	// an IP SAN for an address assigned at scheduling time, so strict
	// verification would report every HTTPS probe as unhealthy. This
	// relaxation never applies to URLs listed in the config file.
	probeClient *http.Client
}

func NewServiceChecker(cfg config.Config, discoverer Discoverer) *ServiceChecker {
	return &ServiceChecker{
		Config:      cfg,
		Discoverer:  discoverer,
		client:      newClient(cfg.Timeout, nil),
		probeClient: newClient(cfg.Timeout, &tls.Config{InsecureSkipVerify: true}),
	}
}

func newClient(timeout time.Duration, tlsConfig *tls.Config) *http.Client {
	return &http.Client{
		Timeout: timeout,
		// Report the endpoint's own status rather than the status of
		// wherever it redirects to.
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Transport: &http.Transport{TLSClientConfig: tlsConfig},
	}
}

// clientFor picks the strict or probe-relaxed client based on where the
// service came from.
func (s *ServiceChecker) clientFor(service config.Service) *http.Client {
	if service.Source == config.SourceKubernetes {
		return s.probeClient
	}
	return s.client
}

// Run checks every service on each tick until ctx is cancelled.
func (s *ServiceChecker) Run(ctx context.Context) {
	ticker := time.NewTicker(s.Config.Interval)
	defer ticker.Stop()

	logger.Info(fmt.Sprintf("Service Sentinel running every %s...", s.Config.Interval))

	s.runRound(ctx)

	for {
		select {
		case <-ctx.Done():
			logger.Info("Shutting down.")
			return
		case <-ticker.C:
			s.runRound(ctx)
		}
	}
}

// runRound resolves the current service list — static config plus anything
// discovered in the cluster — and checks all of it.
func (s *ServiceChecker) runRound(ctx context.Context) {
	services := s.resolveServices(ctx)
	if len(services) == 0 {
		logger.Warn("No services to check this round.")
		return
	}
	s.performHealthChecks(ctx, services)
}

// resolveServices merges statically configured services with freshly
// discovered ones. Discovery runs every round because pods come and go; a
// discovery failure degrades to the static list rather than stopping checks.
func (s *ServiceChecker) resolveServices(ctx context.Context) []config.Service {
	services := make([]config.Service, len(s.Config.Services))
	copy(services, s.Config.Services)

	if s.Discoverer == nil || !s.Config.Kubernetes.Enabled {
		return services
	}

	discovered, err := s.Discoverer.Discover(ctx, s.Config.Kubernetes)
	if err != nil {
		logger.Error(fmt.Sprintf("Kubernetes discovery failed, continuing with %d configured service(s): %v", len(services), err))
		return services
	}

	logger.Info(fmt.Sprintf("Discovered %d probe endpoint(s) in the cluster.", len(discovered)))
	return append(services, discovered...)
}

func (s *ServiceChecker) performHealthChecks(ctx context.Context, services []config.Service) {
	var wg sync.WaitGroup
	results := make(chan string, len(services))

	logger.Info(fmt.Sprintf("Starting health checks for %d service(s)...", len(services)))

	for _, service := range services {
		wg.Add(1)
		go func(service config.Service) {
			defer wg.Done()
			results <- s.checkHealth(ctx, service)
		}(service)
	}

	wg.Wait()
	close(results)

	logger.Info("Health check round completed.")
	for result := range results {
		log.Println(result)
	}
}

// checkHealth performs one request and returns the summary line for it.
func (s *ServiceChecker) checkHealth(ctx context.Context, service config.Service) string {
	start := time.Now()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, service.URL, nil)
	if err != nil {
		message := fmt.Sprintf("Service '%s' (%s) has an invalid URL: %v", service.Name, service.URL, err)
		logger.Error(message)
		return fmt.Sprintf("%s%s: Unhealthy%s", logger.ColorRed, service.Name, logger.ColorReset)
	}

	resp, err := s.clientFor(service).Do(req)
	elapsed := time.Since(start)

	if err != nil {
		// A cancelled context means we are shutting down, not that the
		// service is down — do not page anyone for it.
		if ctx.Err() != nil {
			return fmt.Sprintf("%s: Skipped (shutting down)", service.Name)
		}
		message := fmt.Sprintf("Service '%s' (%s) is Unhealthy! Error: %v (Response time: %v)", service.Name, service.URL, err, elapsed)
		logger.Error(message)
		s.alert(ctx, alert.Event{
			Service:      service,
			Status:       alert.StatusError,
			Reason:       err.Error(),
			ResponseTime: elapsed,
			CheckedAt:    start,
		})
		return fmt.Sprintf("%s%s: Unhealthy%s", logger.ColorRed, service.Name, logger.ColorReset)
	}

	// Drain and close so the connection can be reused instead of leaked.
	defer resp.Body.Close()
	io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))

	if !isHealthy(resp.StatusCode) {
		message := fmt.Sprintf("Service '%s' (%s) returned status code %d (Response time: %v)", service.Name, service.URL, resp.StatusCode, elapsed)
		logger.Warn(message)
		s.alert(ctx, alert.Event{
			Service:      service,
			Status:       alert.StatusUnhealthy,
			Reason:       fmt.Sprintf("HTTP %d %s", resp.StatusCode, http.StatusText(resp.StatusCode)),
			ResponseTime: elapsed,
			CheckedAt:    start,
		})
		return fmt.Sprintf("%s%s: Unhealthy (Status: %d)%s", logger.ColorRed, service.Name, resp.StatusCode, logger.ColorReset)
	}

	logger.Success(fmt.Sprintf("Service '%s' (%s) is Healthy (Response time: %v)", service.Name, service.URL, elapsed))
	return fmt.Sprintf("%s%s: Healthy%s", logger.ColorGreen, service.Name, logger.ColorReset)
}

// alert notifies the configured destinations, unless alert_sources excludes
// this service's origin. The failure is still logged either way.
func (s *ServiceChecker) alert(ctx context.Context, event alert.Event) {
	if !s.Config.ShouldAlert(event.Service) {
		return
	}
	alert.Send(ctx, s.Config, event)
}

// isHealthy treats any 2xx as healthy, matching how the kubelet judges an
// HTTP probe (it accepts 200-399, but we hold endpoints to the stricter 2xx).
func isHealthy(statusCode int) bool {
	return statusCode >= 200 && statusCode < 300
}
