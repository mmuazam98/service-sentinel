package k8s

import (
	"encoding/json"
	"testing"
)

// podFixture mirrors a real API server response closely enough to exercise
// numeric ports, named ports, HTTPS, and non-HTTP probes.
const podFixture = `{
  "items": [
    {
      "metadata": {"name": "api-0", "namespace": "prod"},
      "spec": {"containers": [
        {"name": "api", "readinessProbe": {"httpGet": {"path": "/healthz", "port": 8080}}},
        {"name": "sidecar", "ports": [{"name": "admin", "containerPort": 9901}],
         "readinessProbe": {"httpGet": {"path": "/ready", "port": "admin", "scheme": "HTTPS"}}},
        {"name": "worker", "readinessProbe": {"exec": {"command": ["true"]}}}
      ]},
      "status": {"phase": "Running", "podIP": "10.1.2.3"}
    },
    {
      "metadata": {"name": "pending-0", "namespace": "prod"},
      "spec": {"containers": [{"name": "api", "readinessProbe": {"httpGet": {"port": 80}}}]},
      "status": {"phase": "Pending", "podIP": ""}
    }
  ]
}`

func TestServicesFromPod(t *testing.T) {
	var list podList
	if err := json.Unmarshal([]byte(podFixture), &list); err != nil {
		t.Fatalf("decoding fixture: %v", err)
	}

	var urls []string
	for _, p := range list.Items {
		for _, s := range servicesFromPod(p, "readiness") {
			urls = append(urls, s.URL)
		}
	}

	want := []string{
		"http://10.1.2.3:8080/healthz", // numeric port
		"https://10.1.2.3:9901/ready",  // named port + HTTPS scheme
	}
	if len(urls) != len(want) {
		t.Fatalf("got %d urls %v, want %d", len(urls), urls, len(want))
	}
	for i, w := range want {
		if urls[i] != w {
			t.Errorf("url %d = %q, want %q", i, urls[i], w)
		}
	}
}

func TestServicesFromPodSelectsProbeKind(t *testing.T) {
	var list podList
	if err := json.Unmarshal([]byte(`{"items":[{
      "metadata": {"name": "api-0", "namespace": "prod"},
      "spec": {"containers": [{"name": "api",
        "readinessProbe": {"httpGet": {"path": "/ready", "port": 8080}},
        "livenessProbe":  {"httpGet": {"path": "/live",  "port": 8080}}}]},
      "status": {"phase": "Running", "podIP": "10.1.2.3"}}]}`), &list); err != nil {
		t.Fatalf("decoding fixture: %v", err)
	}

	for kind, wantCount := range map[string]int{"readiness": 1, "liveness": 1, "both": 2} {
		if got := len(servicesFromPod(list.Items[0], kind)); got != wantCount {
			t.Errorf("probe=%s returned %d services, want %d", kind, got, wantCount)
		}
	}
}

// A probe naming a port the container never declares cannot be called.
func TestResolvePortUnknownName(t *testing.T) {
	if _, ok := resolvePort(json.RawMessage(`"nope"`), container{}); ok {
		t.Error("expected unknown named port to be rejected")
	}
}
