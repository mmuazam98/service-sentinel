package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("writing temp config: %v", err)
	}
	return path
}

func TestLoadAppliesDefaults(t *testing.T) {
	cfg, err := Load(writeConfig(t, "services:\n  - name: api\n    url: http://example.test\n"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Interval != time.Minute {
		t.Errorf("interval = %v, want 1m", cfg.Interval)
	}
	if cfg.Timeout != 5*time.Second {
		t.Errorf("timeout = %v, want 5s", cfg.Timeout)
	}
	if cfg.Kubernetes.Probe != ProbeReadiness {
		t.Errorf("probe = %q, want %q", cfg.Kubernetes.Probe, ProbeReadiness)
	}
}

func TestLoadParsesKubernetesBlock(t *testing.T) {
	cfg, err := Load(writeConfig(t, `
interval: 30s
kubernetes:
  enabled: true
  namespaces: [prod, staging]
  label_selector: app=api
  probe: both
`))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Interval != 30*time.Second {
		t.Errorf("interval = %v, want 30s", cfg.Interval)
	}
	if !cfg.Kubernetes.Enabled || cfg.Kubernetes.Probe != ProbeBoth {
		t.Errorf("unexpected kubernetes config: %+v", cfg.Kubernetes)
	}
	if len(cfg.Kubernetes.Namespaces) != 2 {
		t.Errorf("namespaces = %v, want 2 entries", cfg.Kubernetes.Namespaces)
	}
}

func TestLoadRejectsBadConfigs(t *testing.T) {
	cases := map[string]string{
		"no services and no discovery": "services: []\n",
		"missing url":                  "services:\n  - name: api\n",
		"non-http url":                 "services:\n  - name: api\n    url: example.test\n",
		"bad interval":                 "interval: soon\nservices:\n  - {name: api, url: 'http://a.test'}\n",
		"unknown probe":                "kubernetes:\n  enabled: true\n  probe: startup\n",
		"unknown field":                "servicez: []\n",
	}

	for name, body := range cases {
		if _, err := Load(writeConfig(t, body)); err == nil {
			t.Errorf("%s: expected an error, got none", name)
		}
	}
}

func TestLoadEnvOverrides(t *testing.T) {
	t.Setenv("INTERVAL", "10s")
	t.Setenv("SLACK_WEBHOOK_URL", "https://hooks.slack.test/x")
	t.Setenv("KUBERNETES_DISCOVERY", "true")

	cfg, err := Load(writeConfig(t, "interval: 1m\nservices:\n  - {name: api, url: 'http://a.test'}\n"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Interval != 10*time.Second {
		t.Errorf("interval = %v, want 10s from env", cfg.Interval)
	}
	if cfg.SlackWebhookURL != "https://hooks.slack.test/x" {
		t.Errorf("slack url = %q, want the env value", cfg.SlackWebhookURL)
	}
	if !cfg.Kubernetes.Enabled {
		t.Error("expected KUBERNETES_DISCOVERY=true to enable discovery")
	}
}

func TestShouldAlertBySource(t *testing.T) {
	k8sOnly := Config{AlertSources: []string{SourceKubernetes}}
	if k8sOnly.ShouldAlert(Service{Source: SourceConfig}) {
		t.Error("config-sourced service must not alert when alert_sources is [kubernetes]")
	}
	if !k8sOnly.ShouldAlert(Service{Source: SourceKubernetes}) {
		t.Error("kubernetes-sourced service must alert when alert_sources is [kubernetes]")
	}

	// An empty alert_sources keeps the old behaviour: everything alerts.
	all := Config{}
	if !all.ShouldAlert(Service{Source: SourceConfig}) || !all.ShouldAlert(Service{Source: SourceKubernetes}) {
		t.Error("empty alert_sources must allow every source to alert")
	}
}

func TestLoadRejectsUnknownAlertSource(t *testing.T) {
	if _, err := Load(writeConfig(t, "alert_sources: [email]\nservices:\n  - {name: a, url: 'http://a.test'}\n")); err == nil {
		t.Error("expected an error for an unknown alert source")
	}
}
