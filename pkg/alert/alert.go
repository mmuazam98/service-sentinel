package alert

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/mmuazam98/service-sentinel/config"
)

// client is shared so alerts reuse connections and can never hang forever.
var client = &http.Client{Timeout: 10 * time.Second}

// Status describes how a check failed.
type Status string

const (
	// StatusError means the request never completed: DNS, connection
	// refused, TLS failure or timeout.
	StatusError Status = "Error"
	// StatusUnhealthy means the service answered with a non-2xx status.
	StatusUnhealthy Status = "Unhealthy"
)

// Event is everything the notifiers need to describe one failed check.
type Event struct {
	Service config.Service
	Status  Status
	// Reason is the specific cause: an error string, or "HTTP 503".
	Reason       string
	ResponseTime time.Duration
	CheckedAt    time.Time
}

// Send notifies every configured destination. A failing webhook is logged
// and skipped so one broken destination cannot suppress the others.
func Send(ctx context.Context, cfg config.Config, event Event) {
	if cfg.AlertWebhookURL != "" {
		post(ctx, "webhook", cfg.AlertWebhookURL, map[string]string{"text": event.summary()})
	}

	if cfg.SlackWebhookURL != "" {
		post(ctx, "slack", cfg.SlackWebhookURL, buildSlackPayload(event))
	}
}

func post(ctx context.Context, destination, url string, payload any) {
	body, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Error marshalling JSON for %s alert: %v", destination, err)
		return
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		log.Printf("Error building %s alert request: %v", destination, err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error sending alert to %s: %v", destination, err)
		return
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("Alert to %s rejected with status %s", destination, resp.Status)
	}
}
