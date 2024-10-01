package alert

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/mmuazam98/service-sentinel/config"
)

func SendAlert(message string, config config.Config, service config.Service, status string, responseTime time.Duration) {
	if config.AlertWebhookURL != "" {
		payload := map[string]string{"text": message}
		jsonPayload, err := json.Marshal(payload)
		if err != nil {
			log.Printf("Error marshalling JSON for alert: %v", err)
			return
		}

		_, err = http.Post(config.AlertWebhookURL, "application/json", bytes.NewBuffer(jsonPayload))
		if err != nil {
			log.Printf("Error sending alert to webhook: %v", err)
		}
	}

	if config.SlackWebhookURL != "" {
		payload := buildSlackPayload(service.Name, status, responseTime)
		jsonPayload, err := json.Marshal(payload)
		if err != nil {
			log.Printf("Error marshalling JSON for alert: %v", err)
			return
		}

		_, err = http.Post(config.SlackWebhookURL, "application/json", bytes.NewBuffer(jsonPayload))
		if err != nil {
			log.Printf("Error sending alert to slack: %v", err)
		}
	}
}
