package alert

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/mmuazam98/service-sentinel/config"
)

func payloadJSON(t *testing.T, e Event) string {
	t.Helper()
	b, err := json.Marshal(buildSlackPayload(e))
	if err != nil {
		t.Fatalf("marshalling payload: %v", err)
	}
	return string(b)
}

func TestSlackPayloadContainsFailureDetail(t *testing.T) {
	e := Event{
		Service:      config.Service{Name: "default/api/web [readiness]", URL: "https://10.0.0.1:8443/healthz", Source: "kubernetes"},
		Status:       StatusError,
		Reason:       "tls: handshake timeout",
		ResponseTime: 38 * time.Millisecond,
		CheckedAt:    time.Date(2026, 8, 31, 16, 56, 44, 0, time.UTC),
	}

	got := payloadJSON(t, e)
	for _, want := range []string{
		"default/api/web [readiness]",
		"tls: handshake timeout", // the reason must reach Slack
		"https://10.0.0.1:8443/healthz",
		"38ms",
		"Kubernetes",
		"is unreachable",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("payload missing %q\npayload: %s", want, got)
		}
	}
}

// The product name belongs in the grey context footer, not a bold header.
func TestSlackPayloadHasNoHeaderBlock(t *testing.T) {
	e := Event{Service: config.Service{Name: "api"}, Status: StatusUnhealthy, Reason: "HTTP 503"}

	var payload struct {
		Blocks []struct {
			Type     string `json:"type"`
			Elements []struct {
				Text string `json:"text"`
			} `json:"elements"`
		} `json:"blocks"`
	}
	if err := json.Unmarshal([]byte(payloadJSON(t, e)), &payload); err != nil {
		t.Fatalf("unmarshalling payload: %v", err)
	}

	last := payload.Blocks[len(payload.Blocks)-1]
	for _, b := range payload.Blocks {
		if b.Type == "header" {
			t.Error("payload should not contain a header block")
		}
	}
	if last.Type != "context" || !strings.Contains(last.Elements[0].Text, "Service Sentinel") {
		t.Errorf("expected the branding in a trailing context block, got %+v", last)
	}
}

func TestFormatDuration(t *testing.T) {
	cases := map[time.Duration]string{
		38 * time.Millisecond:   "38ms",
		1500 * time.Millisecond: "1.50s",
	}
	for d, want := range cases {
		if got := formatDuration(d); got != want {
			t.Errorf("formatDuration(%v) = %q, want %q", d, got, want)
		}
	}
}

// A backtick in an error must not break out of the inline code span.
func TestCodeEscapesBackticks(t *testing.T) {
	if strings.Contains(code("weird `injected` value")[1:len(code("weird `injected` value"))-1], "`") {
		t.Error("backticks inside a code span were not escaped")
	}
}
