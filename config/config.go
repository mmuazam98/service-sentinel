package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v2"
)

// DefaultPath is used when neither -config nor CONFIG_PATH is set.
const DefaultPath = "config/config.yaml"

type Config struct {
	// IntervalRaw/TimeoutRaw are the YAML-facing strings ("30s", "2m").
	// yaml.v2 cannot decode a duration string into time.Duration, so the
	// parsed values live in Interval/Timeout, filled by Load.
	IntervalRaw string `yaml:"interval"`
	TimeoutRaw  string `yaml:"timeout"`

	Interval time.Duration `yaml:"-"`
	Timeout  time.Duration `yaml:"-"`

	Services   []Service  `yaml:"services"`
	Kubernetes Kubernetes `yaml:"kubernetes"`

	AlertWebhookURL string `yaml:"alert_webhook_url,omitempty"`
	SlackWebhookURL string `yaml:"slack_webhook_url,omitempty"`

	// AlertSources limits which services may raise alerts, by origin:
	// "kubernetes" for discovered probes, "config" for the services listed
	// above. Empty means every source alerts.
	AlertSources []string `yaml:"alert_sources"`
}

// Service origins, used by AlertSources.
const (
	SourceConfig     = "config"
	SourceKubernetes = "kubernetes"
)

// ShouldAlert reports whether a failure of this service may raise an alert.
func (c Config) ShouldAlert(s Service) bool {
	if len(c.AlertSources) == 0 {
		return true
	}
	for _, source := range c.AlertSources {
		if source == s.Source {
			return true
		}
	}
	return false
}

type Service struct {
	Name string `yaml:"name"`
	URL  string `yaml:"url"`
	// Source is "config" for statically listed services and "kubernetes"
	// for ones found by probe discovery. Set by the discoverer, not YAML.
	Source string `yaml:"-"`
}

// Kubernetes controls in-cluster discovery of pods that declare an HTTP
// readiness or liveness probe.
type Kubernetes struct {
	Enabled bool `yaml:"enabled"`
	// Namespaces to scan. Empty means all namespaces the service account
	// is allowed to list.
	Namespaces []string `yaml:"namespaces"`
	// LabelSelector filters pods, e.g. "app=api,tier!=batch".
	LabelSelector string `yaml:"label_selector"`
	// Probe selects which probe to call: "readiness", "liveness" or "both".
	// Defaults to "readiness".
	Probe string `yaml:"probe"`
}

const (
	ProbeReadiness = "readiness"
	ProbeLiveness  = "liveness"
	ProbeBoth      = "both"
)

// Load reads the config file at path, applies env overrides and defaults,
// and validates the result.
func Load(path string) (Config, error) {
	if path == "" {
		path = DefaultPath
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("reading config %q: %w", path, err)
	}

	var cfg Config
	// UnmarshalStrict surfaces typos in the config file instead of
	// silently ignoring them.
	if err := yaml.UnmarshalStrict(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parsing config %q: %w", path, err)
	}

	if v := os.Getenv("ALERT_WEBHOOK_URL"); v != "" {
		cfg.AlertWebhookURL = v
	}
	if v := os.Getenv("SLACK_WEBHOOK_URL"); v != "" {
		cfg.SlackWebhookURL = v
	}
	if v := os.Getenv("INTERVAL"); v != "" {
		cfg.IntervalRaw = v
	}
	if v := os.Getenv("TIMEOUT"); v != "" {
		cfg.TimeoutRaw = v
	}
	if v := os.Getenv("KUBERNETES_DISCOVERY"); v != "" {
		cfg.Kubernetes.Enabled = strings.EqualFold(v, "true") || v == "1"
	}

	if err := cfg.applyDefaults(); err != nil {
		return Config{}, err
	}
	if err := cfg.validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func (c *Config) applyDefaults() error {
	var err error

	if c.Interval, err = parseDuration(c.IntervalRaw, time.Minute); err != nil {
		return fmt.Errorf("invalid interval %q: %w", c.IntervalRaw, err)
	}
	if c.Timeout, err = parseDuration(c.TimeoutRaw, 5*time.Second); err != nil {
		return fmt.Errorf("invalid timeout %q: %w", c.TimeoutRaw, err)
	}

	if c.Kubernetes.Probe == "" {
		c.Kubernetes.Probe = ProbeReadiness
	}
	c.Kubernetes.Probe = strings.ToLower(c.Kubernetes.Probe)

	for i := range c.Services {
		c.Services[i].Source = SourceConfig
		if c.Services[i].Name == "" {
			c.Services[i].Name = c.Services[i].URL
		}
	}

	return nil
}

func (c *Config) validate() error {
	if len(c.Services) == 0 && !c.Kubernetes.Enabled {
		return fmt.Errorf("no services configured and kubernetes discovery is disabled")
	}

	for _, s := range c.Services {
		if s.URL == "" {
			return fmt.Errorf("service %q has no url", s.Name)
		}
		if !strings.HasPrefix(s.URL, "http://") && !strings.HasPrefix(s.URL, "https://") {
			return fmt.Errorf("service %q url must start with http:// or https://, got %q", s.Name, s.URL)
		}
	}

	switch c.Kubernetes.Probe {
	case ProbeReadiness, ProbeLiveness, ProbeBoth:
	default:
		return fmt.Errorf("kubernetes.probe must be one of %q, %q, %q; got %q",
			ProbeReadiness, ProbeLiveness, ProbeBoth, c.Kubernetes.Probe)
	}

	for _, source := range c.AlertSources {
		if source != SourceConfig && source != SourceKubernetes {
			return fmt.Errorf("alert_sources may only contain %q or %q; got %q",
				SourceConfig, SourceKubernetes, source)
		}
	}

	if c.Interval <= 0 {
		return fmt.Errorf("interval must be positive, got %v", c.Interval)
	}
	if c.Timeout <= 0 {
		return fmt.Errorf("timeout must be positive, got %v", c.Timeout)
	}

	return nil
}

func parseDuration(raw string, fallback time.Duration) (time.Duration, error) {
	if raw == "" {
		return fallback, nil
	}
	return time.ParseDuration(raw)
}
