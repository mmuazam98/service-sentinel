package checker

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mmuazam98/service-sentinel/config"
)

func TestCheckHealth(t *testing.T) {
	// Mock HTTP server for healthy service
	mockServerHealthy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer mockServerHealthy.Close()

	// Mock HTTP server for unhealthy service
	mockServerUnhealthy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer mockServerUnhealthy.Close()

	services := []config.Service{
		{Name: "HealthyService", URL: mockServerHealthy.URL},
		{Name: "UnhealthyService", URL: mockServerUnhealthy.URL},
	}

	config := config.Config{
		Services:        services,
		AlertWebhookURL: "",
	}

	serviceChecker := NewServiceChecker(config)
	var logOutput bytes.Buffer
	log.SetOutput(&logOutput)

	serviceChecker.performHealthChecks(services)

	output := logOutput.String()

	if !strings.Contains(output, "HealthyService: Healthy") {
		t.Errorf("Expected 'HealthyService: Healthy' in logs, but got: %s", output)
	}
	if !strings.Contains(output, "UnhealthyService: Unhealthy") {
		t.Errorf("Expected 'UnhealthyService: Unhealthy' in logs, but got: %s", output)
	}
}
