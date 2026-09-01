package alert

import (
	"fmt"
	"strings"
	"time"
)

// summary is the one-line form used by the generic webhook.
func (e Event) summary() string {
	return fmt.Sprintf("[%s] %s — %s (%s, response time %s)",
		e.Status, e.Service.Name, e.Reason, e.Service.URL, formatDuration(e.ResponseTime))
}

// emoji gives an at-a-glance severity marker in the Slack message.
func (e Event) emoji() string {
	if e.Status == StatusError {
		return ":red_circle:"
	}
	return ":large_orange_diamond:"
}

// origin labels where the service came from, so it is obvious whether an
// alert concerns a discovered pod or a hand-configured endpoint.
func (e Event) origin() string {
	if e.Service.Source == "kubernetes" {
		return "Kubernetes"
	}
	return "Config"
}

// buildSlackPayload renders a compact Block Kit message: a headline that says
// what broke, a field grid for the details, the failure reason in a code
// block, and the product name demoted to a grey context line at the bottom.
func buildSlackPayload(e Event) map[string]any {
	checkedAt := e.CheckedAt
	if checkedAt.IsZero() {
		checkedAt = time.Now()
	}

	blocks := []map[string]any{
		{
			"type": "section",
			"text": map[string]any{
				"type": "mrkdwn",
				"text": fmt.Sprintf("%s *%s*\n%s", e.emoji(), e.Service.Name, statusLine(e)),
			},
		},
		{
			"type": "section",
			"fields": []map[string]any{
				mrkdwn("*Status*\n" + string(e.Status)),
				mrkdwn("*Response time*\n" + formatDuration(e.ResponseTime)),
				mrkdwn("*Source*\n" + e.origin()),
				mrkdwn("*Endpoint*\n" + code(e.Service.URL)),
			},
		},
	}

	if e.Reason != "" {
		blocks = append(blocks, map[string]any{
			"type": "section",
			"text": mrkdwn("*Reason*\n" + code(truncate(e.Reason, 900))),
		})
	}

	blocks = append(blocks, map[string]any{
		"type": "context",
		"elements": []map[string]any{
			mrkdwn(fmt.Sprintf("Service Sentinel  ·  checked at %s", checkedAt.Format("2006-01-02 15:04:05 MST"))),
		},
	})

	return map[string]any{
		// text is the notification/preview line shown in the sidebar and in
		// push notifications, where blocks are not rendered.
		"text":   fmt.Sprintf("%s %s %s", e.emoji(), e.Service.Name, e.headline()),
		"blocks": blocks,
	}
}

// headline is the short predicate used in the notification preview.
func (e Event) headline() string {
	if e.Status == StatusError {
		return "is unreachable"
	}
	return "is unhealthy"
}

func statusLine(e Event) string {
	if e.Status == StatusError {
		return "Health check failed — the endpoint could not be reached."
	}
	return "Health check failed — the endpoint returned an unhealthy response."
}

func mrkdwn(text string) map[string]any {
	return map[string]any{"type": "mrkdwn", "text": text}
}

// code wraps text in Slack inline code, escaping any backtick that would
// otherwise break out of the span.
func code(text string) string {
	return "`" + strings.ReplaceAll(text, "`", "'") + "`"
}

// formatDuration keeps timings readable: milliseconds for fast checks,
// seconds once a check starts dragging.
func formatDuration(d time.Duration) string {
	if d >= time.Second {
		return fmt.Sprintf("%.2fs", d.Seconds())
	}
	return fmt.Sprintf("%dms", d.Milliseconds())
}

// truncate keeps a long error from blowing past Slack's 3000 character
// per-block limit.
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}
