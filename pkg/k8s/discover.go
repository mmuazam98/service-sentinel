// Package k8s discovers in-cluster pods that declare an HTTP readiness or
// liveness probe and turns those probes into checkable URLs.
//
// It talks to the API server directly over HTTPS using the projected
// service account credentials, so it pulls in no client-go dependency.
package k8s

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/mmuazam98/service-sentinel/config"
)

const (
	tokenFile     = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	caFile        = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
	namespaceFile = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
)

// Discoverer lists pods from the Kubernetes API server.
type Discoverer struct {
	apiServer string
	client    *http.Client
	// tokenPath is read on every request rather than cached, because
	// projected service account tokens are rotated on disk.
	tokenPath string
}

// InCluster builds a Discoverer from the service account mounted into the
// pod. It fails if the process is not running inside a cluster.
func InCluster() (*Discoverer, error) {
	host, port := os.Getenv("KUBERNETES_SERVICE_HOST"), os.Getenv("KUBERNETES_SERVICE_PORT")
	if host == "" || port == "" {
		return nil, fmt.Errorf("not running in a cluster: KUBERNETES_SERVICE_HOST/PORT unset")
	}

	ca, err := os.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("reading cluster CA: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(ca) {
		return nil, fmt.Errorf("cluster CA at %s contains no valid certificates", caFile)
	}

	return &Discoverer{
		apiServer: "https://" + net.JoinHostPort(host, port),
		tokenPath: tokenFile,
		client: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12},
			},
		},
	}, nil
}

// Namespace returns the namespace this pod runs in.
func Namespace() string {
	b, err := os.ReadFile(namespaceFile)
	if err != nil {
		return "default"
	}
	return string(b)
}

// Discover returns one service per HTTP probe found on a running pod,
// filtered by the namespaces, label selector and probe kind in cfg.
func (d *Discoverer) Discover(ctx context.Context, cfg config.Kubernetes) ([]config.Service, error) {
	namespaces := cfg.Namespaces
	if len(namespaces) == 0 {
		namespaces = []string{""} // empty means cluster-wide
	}

	var services []config.Service
	for _, ns := range namespaces {
		pods, err := d.listPods(ctx, ns, cfg.LabelSelector)
		if err != nil {
			return nil, err
		}
		for _, pod := range pods {
			services = append(services, servicesFromPod(pod, cfg.Probe)...)
		}
	}

	return services, nil
}

func (d *Discoverer) listPods(ctx context.Context, namespace, labelSelector string) ([]pod, error) {
	path := "/api/v1/pods"
	if namespace != "" {
		path = "/api/v1/namespaces/" + url.PathEscape(namespace) + "/pods"
	}

	endpoint := d.apiServer + path
	if labelSelector != "" {
		endpoint += "?labelSelector=" + url.QueryEscape(labelSelector)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	token, err := os.ReadFile(d.tokenPath)
	if err != nil {
		return nil, fmt.Errorf("reading service account token: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+string(token))
	req.Header.Set("Accept", "application/json")

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("listing pods: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("listing pods in namespace %q: API server returned %s "+
			"(does the service account have list access to pods?)", namespace, resp.Status)
	}

	var list podList
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return nil, fmt.Errorf("decoding pod list: %w", err)
	}

	return list.Items, nil
}

// servicesFromPod converts every matching HTTP probe on a running pod into a
// checkable service. Pods without an IP, or with non-HTTP probes (exec, tcp),
// are skipped.
func servicesFromPod(p pod, probeKind string) []config.Service {
	if p.Status.Phase != "Running" || p.Status.PodIP == "" {
		return nil
	}

	var services []config.Service
	for _, c := range p.Spec.Containers {
		for _, probe := range selectProbes(c, probeKind) {
			url, ok := probeURL(probe.spec, c, p.Status.PodIP)
			if !ok {
				continue
			}
			services = append(services, config.Service{
				Name:   fmt.Sprintf("%s/%s/%s [%s]", p.Metadata.Namespace, p.Metadata.Name, c.Name, probe.kind),
				URL:    url,
				Source: config.SourceKubernetes,
			})
		}
	}

	return services
}

type namedProbe struct {
	kind string
	spec *probe
}

func selectProbes(c container, probeKind string) []namedProbe {
	var probes []namedProbe
	if probeKind == config.ProbeReadiness || probeKind == config.ProbeBoth {
		if c.ReadinessProbe != nil {
			probes = append(probes, namedProbe{config.ProbeReadiness, c.ReadinessProbe})
		}
	}
	if probeKind == config.ProbeLiveness || probeKind == config.ProbeBoth {
		if c.LivenessProbe != nil {
			probes = append(probes, namedProbe{config.ProbeLiveness, c.LivenessProbe})
		}
	}
	return probes
}

// probeURL builds the URL for an httpGet probe, mirroring how the kubelet
// resolves it: scheme defaults to HTTP, host to the pod IP, and a named port
// is looked up in the container's port list.
func probeURL(pr *probe, c container, podIP string) (string, bool) {
	if pr.HTTPGet == nil {
		return "", false // exec or tcpSocket probe, nothing to call over HTTP
	}
	get := pr.HTTPGet

	scheme := "http"
	if get.Scheme == "HTTPS" {
		scheme = "https"
	}

	port, ok := resolvePort(get.Port, c)
	if !ok {
		return "", false
	}

	host := get.Host
	if host == "" {
		host = podIP
	}

	path := get.Path
	if path == "" {
		path = "/"
	}

	return scheme + "://" + net.JoinHostPort(host, strconv.Itoa(port)) + path, true
}

// resolvePort handles the IntOrString port field: a number is used as-is, a
// name is matched against the container's declared ports.
func resolvePort(raw json.RawMessage, c container) (int, bool) {
	if len(raw) == 0 {
		return 0, false
	}

	var number int
	if err := json.Unmarshal(raw, &number); err == nil {
		return number, number > 0
	}

	var name string
	if err := json.Unmarshal(raw, &name); err != nil {
		return 0, false
	}
	for _, p := range c.Ports {
		if p.Name == name {
			return p.ContainerPort, true
		}
	}

	return 0, false
}

// Minimal mirrors of the Kubernetes API types — only the fields probe
// discovery reads. Adding a field here is cheaper than depending on client-go.
type podList struct {
	Items []pod `json:"items"`
}

type pod struct {
	Metadata struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
	} `json:"metadata"`
	Spec struct {
		Containers []container `json:"containers"`
	} `json:"spec"`
	Status struct {
		Phase string `json:"phase"`
		PodIP string `json:"podIP"`
	} `json:"status"`
}

type container struct {
	Name  string `json:"name"`
	Ports []struct {
		Name          string `json:"name"`
		ContainerPort int    `json:"containerPort"`
	} `json:"ports"`
	ReadinessProbe *probe `json:"readinessProbe"`
	LivenessProbe  *probe `json:"livenessProbe"`
}

type probe struct {
	HTTPGet *struct {
		Path   string          `json:"path"`
		Port   json.RawMessage `json:"port"` // int or string
		Host   string          `json:"host"`
		Scheme string          `json:"scheme"`
	} `json:"httpGet"`
}
