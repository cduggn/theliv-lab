package prometheus

import (
	"fmt"
	"strings"
)

// The Kubernetes API connection stays HTTPS; scheme controls its upstream service.
func serviceProxyAddress(host, namespace, name, port, scheme string) (string, error) {
	if scheme == "" {
		scheme = "https"
	}
	if scheme != "http" && scheme != "https" {
		return "", fmt.Errorf("unsupported Prometheus service scheme %q", scheme)
	}
	if host == "" || namespace == "" || name == "" || port == "" {
		return "", fmt.Errorf("Prometheus service proxy requires host, namespace, name and port")
	}
	return fmt.Sprintf("%s/api/v1/namespaces/%s/services/%s:%s:%s/proxy",
		strings.TrimRight(host, "/"), namespace, scheme, name, port), nil
}
