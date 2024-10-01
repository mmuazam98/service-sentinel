package checker

import (
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/mmuazam98/service-sentinel/config"
	"github.com/mmuazam98/service-sentinel/pkg/alert"
	"github.com/mmuazam98/service-sentinel/pkg/logger"
)

type ServiceChecker struct {
	Config config.Config
}

func NewServiceChecker(config config.Config) *ServiceChecker {
	return &ServiceChecker{
		Config: config,
	}
}

func (s *ServiceChecker) Run(intervalMinutes int) {
	ticker := time.NewTicker(time.Duration(intervalMinutes) * time.Minute)
	defer ticker.Stop()

	logger.Info(fmt.Sprintf("Service Sentinel running every %d minutes...", intervalMinutes))

	s.performHealthChecks(s.Config.Services)

	for {
		select {
		case <-ticker.C:
			s.performHealthChecks(s.Config.Services)
		}
	}
}

func (s *ServiceChecker) performHealthChecks(services []config.Service) {
	var wg sync.WaitGroup
	results := make(chan string, len(services))

	logger.Info("Starting health checks...")

	for _, service := range services {
		wg.Add(1)
		go s.checkHealth(service, &wg, results)
	}

	wg.Wait()
	close(results)

	logger.Info("Health check round completed.")
	for result := range results {
		log.Println(result)
	}
}

func (s *ServiceChecker) checkHealth(service config.Service, wg *sync.WaitGroup, results chan<- string) {
	defer wg.Done()

	start := time.Now()

	client := http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := client.Get(service.URL)
	elapsed := time.Since(start)

	if err != nil {
		message := fmt.Sprintf("Service '%s' (%s) is Unhealthy! Error: %v (Response time: %v)", service.Name, service.URL, err, elapsed)
		logger.Error(message)
		results <- fmt.Sprintf("%s%s: Unhealthy%s", logger.ColorRed, service.Name, logger.ColorReset)
		alert.SendAlert(message, s.Config, service, "Error", elapsed)
		return
	}

	if resp.StatusCode != http.StatusOK {
		message := fmt.Sprintf("Service '%s' (%s) returned status code %d (Response time: %v)", service.Name, service.URL, resp.StatusCode, elapsed)
		logger.Warn(message)
		results <- fmt.Sprintf("%s%s: Unhealthy (Status: %d)%s", logger.ColorRed, service.Name, resp.StatusCode, logger.ColorReset)
		alert.SendAlert(message, s.Config, service, "Unhealthy", elapsed)
		return
	}

	logger.Success(fmt.Sprintf("Service '%s' (%s) is Healthy (Response time: %v)", service.Name, service.URL, elapsed))
	results <- fmt.Sprintf("%s%s: Healthy%s", logger.ColorGreen, service.Name, logger.ColorReset)
}
