# Service Sentinel

![Go](https://img.shields.io/badge/Language-Go-blue.svg)
![License](https://img.shields.io/badge/License-MIT-green.svg)

**Service Sentinel** is an open-source service health checker built with Go. It periodically calls the health endpoints of your services, logs colour-coded results, and alerts you when something breaks.

Point it at a list of URLs, drop it into a Kubernetes cluster to have it find your readiness and liveness probes on its own, or do both at once.

## Features

- Periodic health checks of configured services, run concurrently
- **Kubernetes discovery**: finds pods with HTTP readiness/liveness probes and checks them automatically
- Alerts to a generic webhook and/or Slack
- Colour-coded logs with response times
- Configured by a single YAML file, overridable by environment variables
- Graceful shutdown on `SIGINT`/`SIGTERM`
- No third-party dependencies beyond a YAML parser

## Getting Started

### Prerequisites

- [Go](https://golang.org/doc/install) 1.22 or newer
- A YAML config file

### Installation

```bash
git clone https://github.com/mmuazam98/service-sentinel.git
cd service-sentinel
go mod tidy
go run ./cmd -config config/config.yaml
```

## Configuration

```yaml
# How often to check every service, and how long a single check may take.
interval: 1m
timeout: 5s

# Statically configured endpoints. These are always checked.
services:
  - name: "GitHub API"
    url: "https://api.github.com"
  - name: "Unhealthy 500"
    url: "https://httpstat.us/500"

# In-cluster discovery of readiness/liveness probes.
kubernetes:
  enabled: false
  namespaces: []        # empty = every namespace the service account can list
  label_selector: ""    # same syntax as `kubectl get pods -l`
  probe: readiness      # readiness | liveness | both

alert_webhook_url: ""
slack_webhook_url: ""
```

A service is **healthy** when it answers with any `2xx` status inside the timeout. Anything else — a non-`2xx` status, a timeout, a connection error — is unhealthy and triggers an alert.

### Environment variables

Useful for containers, where you do not want secrets baked into the config file.

| Variable | Effect |
| --- | --- |
| `CONFIG_PATH` | Path to the config file (default `config/config.yaml`) |
| `INTERVAL` | Overrides `interval`, e.g. `30s` |
| `TIMEOUT` | Overrides `timeout` |
| `KUBERNETES_DISCOVERY` | Set to `true` to enable discovery |
| `ALERT_WEBHOOK_URL` | Generic webhook destination |
| `SLACK_WEBHOOK_URL` | Slack incoming webhook |

### Flags

| Flag | Effect |
| --- | --- |
| `-config` | Path to the config file |
| `-interval` | Overrides the check interval, e.g. `-interval 30s` |

Precedence is **flags > environment variables > config file**.

## Kubernetes discovery

With `kubernetes.enabled: true`, every round Service Sentinel asks the API server for pods, reads the `httpGet` readiness and/or liveness probes off their containers, and turns each one into a URL it checks:

```
readinessProbe:            ->  http://<pod-ip>:8080/healthz
  httpGet:
    path: /healthz
    port: 8080
```

Details worth knowing:

- Discovery re-runs on **every interval**, so pods that scale up or roll are picked up without a restart.
- Named ports (`port: admin`) are resolved against the container's declared ports, and `scheme: HTTPS` is honoured — exactly as the kubelet does it.
- Only `Running` pods that have an IP are checked. `exec` and `tcpSocket` probes are skipped, since there is nothing to call over HTTP.
- Discovered services are checked **in addition to** the ones in `services:`, never instead of them.
- If the API server is unreachable, the round logs the error and continues with the statically configured services.

Discovery talks to the API server directly using the pod's service account token, so there is no `client-go` dependency and nothing to configure beyond RBAC.

### Deploying

```bash
kubectl apply -f deploy/kubernetes.yaml
```

That manifest creates a namespace, a service account with **read-only access to pods**, a ConfigMap holding the config, and a Deployment. Alert webhooks are read from an optional Secret:

```bash
kubectl -n service-sentinel create secret generic service-sentinel-alerts \
  --from-literal=slack-webhook-url=https://hooks.slack.com/services/...
```

If you scope `kubernetes.namespaces` to a fixed list, you can downgrade the `ClusterRole` in the manifest to a namespaced `Role`.

## Docker

```bash
docker build -t service-sentinel .
docker run --rm \
  -v "$PWD/config/config.yaml:/etc/service-sentinel/config.yaml:ro" \
  -e CONFIG_PATH=/etc/service-sentinel/config.yaml \
  service-sentinel
```

The image is a static binary on `distroless/static`, running as non-root.

## Development

```bash
go test ./...
go vet ./...
```

## Project layout

```
cmd/            entrypoint, flag parsing, signal handling
config/         config loading, defaults, validation
pkg/checker/    the health check loop
pkg/k8s/        Kubernetes probe discovery
pkg/alert/      webhook and Slack notifications
pkg/logger/     colour-coded logging
deploy/         Kubernetes manifests
```

## Contributing

Issues and pull requests are welcome. Please run `go test ./...` and `gofmt -l .` before opening a PR.

## License

MIT — see [LICENSE](LICENSE).
