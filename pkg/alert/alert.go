package alert

import (
	"bytes"
	"encoding/json"
	"fmt"
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
		alertMessage := map[string]interface{}{
			"blocks": []map[string]interface{}{
				{
					"type": "header",
					"text": map[string]interface{}{
						"type":  "plain_text",
						"text":  "Service Sentinel",
						"emoji": true,
					},
				},
				{
					"type": "section",
					"text": map[string]string{
						"type": "mrkdwn",
						"text": fmt.Sprintf("*Service:* %s\n*Status:* %s\n*Response Time:* %v", service.Name, status, responseTime),
					},
				},
				{
					"type": "divider",
				},
				{
					"type": "section",
					"text": map[string]string{
						"type": "mrkdwn",
						"text": "Check the service for more details! :warning:",
					},
				},
				{
					"type": "context",
					"elements": []map[string]string{
						{
							"type": "plain_text",
							"text": fmt.Sprintf("Checked at: %s", time.Now().Format("2006-01-02 15:04:05")),
						},
					},
				},
			},
		}

		jsonPayload, err := json.Marshal(alertMessage)
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
