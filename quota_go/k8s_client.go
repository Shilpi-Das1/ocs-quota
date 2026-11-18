package quota

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// K8sQuotaClient reads ResourceQuota objects from the Kubernetes REST API.
type K8sQuotaClient struct {
	apiURL    string
	token     string
	namespace string
	http      *http.Client
}

// NewK8sQuotaClient creates a client for the given cluster API URL and namespace.
func NewK8sQuotaClient(apiURL, token, namespace string) *K8sQuotaClient {
	return &K8sQuotaClient{
		apiURL:    strings.TrimRight(apiURL, "/"),
		token:     token,
		namespace: namespace,
		http:      &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *K8sQuotaClient) get(path string) (map[string]interface{}, error) {
	req, err := http.NewRequest(http.MethodGet, c.apiURL+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("K8s API returned HTTP %d for %s", resp.StatusCode, path)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, err
	}
	return data, nil
}

// GetHardLimitGiB returns the hard storage limit (requests.storage) in GiB.
func (c *K8sQuotaClient) GetHardLimitGiB(quotaName string) (float64, error) {
	path := fmt.Sprintf("/api/v1/namespaces/%s/resourcequotas/%s", c.namespace, quotaName)
	data, err := c.get(path)
	if err != nil {
		return 0, err
	}
	raw := nestedString(data, "spec", "hard", "requests.storage")
	return ParseQuantityGiB(raw)
}

// GetUsedGiB returns the currently-used storage in GiB from a ResourceQuota status.
func (c *K8sQuotaClient) GetUsedGiB(quotaName string) (float64, error) {
	path := fmt.Sprintf("/api/v1/namespaces/%s/resourcequotas/%s", c.namespace, quotaName)
	data, err := c.get(path)
	if err != nil {
		return 0, err
	}
	raw := nestedString(data, "status", "used", "requests.storage")
	return ParseQuantityGiB(raw)
}

// ParseQuantityGiB converts a Kubernetes quantity string (e.g. "10Gi", "512Mi") to GiB.
func ParseQuantityGiB(quantity string) (float64, error) {
	quantity = strings.TrimSpace(quantity)
	switch {
	case strings.HasSuffix(quantity, "Gi"):
		var v float64
		_, err := fmt.Sscanf(quantity[:len(quantity)-2], "%f", &v)
		return v, err
	case strings.HasSuffix(quantity, "Mi"):
		var v float64
		_, err := fmt.Sscanf(quantity[:len(quantity)-2], "%f", &v)
		return v / 1024, err
	case strings.HasSuffix(quantity, "Ki"):
		var v float64
		_, err := fmt.Sscanf(quantity[:len(quantity)-2], "%f", &v)
		return v / (1024 * 1024), err
	case strings.HasSuffix(quantity, "G"):
		var v float64
		_, err := fmt.Sscanf(quantity[:len(quantity)-1], "%f", &v)
		return v * 1e9 / (1024 * 1024 * 1024), err
	default:
		var v float64
		_, err := fmt.Sscanf(quantity, "%f", &v)
		return v / (1024 * 1024 * 1024), err
	}
}

// nestedString walks a nested map and returns the string value at the key path.
// Returns "0" if any key is missing or the value is not a string.
func nestedString(m map[string]interface{}, keys ...string) string {
	var cur interface{} = m
	for _, k := range keys {
		mp, ok := cur.(map[string]interface{})
		if !ok {
			return "0"
		}
		cur = mp[k]
	}
	s, ok := cur.(string)
	if !ok {
		return "0"
	}
	return s
}
