package alert

import (
	"fmt"
	"time"
)

func buildSlackPayload(serviceName, status string, responseTime time.Duration) map[string]interface{} {
	return map[string]interface{}{
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
					"text": fmt.Sprintf("*Service:* %s\n*Status:* %s\n*Response Time:* %v", serviceName, status, responseTime),
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
}
