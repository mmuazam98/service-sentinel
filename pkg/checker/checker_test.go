package checker

import (
	"bytes"
	"context"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mmuazam98/service-sentinel/config"
)

func TestCheckHealth(t *testing.T) {
	healthy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer healthy.Close()

	// 204 is a 2xx, so it must count as healthy just like 200.
	noContent := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer noContent.Close()

	unhealthy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer unhealthy.Close()

	services := []config.Service{
		{Name: "HealthyService", URL: healthy.URL},
		{Name: "NoContentService", URL: noContent.URL},
		{Name: "UnhealthyService", URL: unhealthy.URL},
	}

	checker := NewServiceChecker(config.Config{Services: services, Timeout: 5 * time.Second}, nil)

	var logOutput bytes.Buffer
	log.SetOutput(&logOutput)
	defer log.SetOutput(os.Stderr)

	checker.performHealthChecks(context.Background(), services)

	output := logOutput.String()
	for _, want := range []string{
		"HealthyService: Healthy",
		"NoContentService: Healthy",
		"UnhealthyService: Unhealthy (Status: 500)",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("expected %q in logs, got: %s", want, output)
		}
	}
}

func TestIsHealthy(t *testing.T) {
	cases := map[int]bool{200: true, 204: true, 299: true, 301: false, 404: false, 500: false}
	for status, want := range cases {
		if got := isHealthy(status); got != want {
			t.Errorf("isHealthy(%d) = %v, want %v", status, got, want)
		}
	}
}

type stubDiscoverer struct {
	services []config.Service
	err      error
}

func (s stubDiscoverer) Discover(context.Context, config.Kubernetes) ([]config.Service, error) {
	return s.services, s.err
}

func TestResolveServicesMergesDiscovered(t *testing.T) {
	cfg := config.Config{
		Services:   []config.Service{{Name: "static", URL: "http://example.test"}},
		Kubernetes: config.Kubernetes{Enabled: true},
	}
	discovered := []config.Service{{Name: "pod", URL: "http://10.0.0.1:8080/healthz"}}

	checker := NewServiceChecker(cfg, stubDiscoverer{services: discovered})

	got := checker.resolveServices(context.Background())
	if len(got) != 2 || got[0].Name != "static" || got[1].Name != "pod" {
		t.Fatalf("expected static + discovered services, got %+v", got)
	}
}

// A discovery outage must not stop the statically configured checks.
func TestResolveServicesFallsBackWhenDiscoveryFails(t *testing.T) {
	cfg := config.Config{
		Services:   []config.Service{{Name: "static", URL: "http://example.test"}},
		Kubernetes: config.Kubernetes{Enabled: true},
	}

	checker := NewServiceChecker(cfg, stubDiscoverer{err: errors.New("api server down")})

	got := checker.resolveServices(context.Background())
	if len(got) != 1 || got[0].Name != "static" {
		t.Fatalf("expected only the static service, got %+v", got)
	}
}

// Discovery results must never leak back into the configured slice.
func TestResolveServicesDoesNotMutateConfig(t *testing.T) {
	cfg := config.Config{
		Services:   []config.Service{{Name: "static", URL: "http://example.test"}},
		Kubernetes: config.Kubernetes{Enabled: true},
	}
	checker := NewServiceChecker(cfg, stubDiscoverer{services: []config.Service{{Name: "pod"}}})

	checker.resolveServices(context.Background())

	if len(checker.Config.Services) != 1 {
		t.Fatalf("configured services were mutated: %+v", checker.Config.Services)
	}
}

// Discovered probes hit pod IPs whose certificates cannot have IP SANs, so
// they must skip verification — exactly as the kubelet does. Config services
// must keep strict verification.
func TestTLSVerificationDependsOnSource(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	checker := NewServiceChecker(config.Config{Timeout: 5 * time.Second}, nil)

	discovered := config.Service{Name: "pod", URL: server.URL, Source: config.SourceKubernetes}
	if got := checker.checkHealth(context.Background(), discovered); !strings.Contains(got, "Healthy") || strings.Contains(got, "Unhealthy") {
		t.Errorf("discovered probe over self-signed TLS should be healthy, got %q", got)
	}

	fromConfig := config.Service{Name: "external", URL: server.URL, Source: config.SourceConfig}
	if got := checker.checkHealth(context.Background(), fromConfig); !strings.Contains(got, "Unhealthy") {
		t.Errorf("config service with an untrusted certificate should be unhealthy, got %q", got)
	}
}
