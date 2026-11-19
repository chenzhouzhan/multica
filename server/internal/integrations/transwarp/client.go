// Package transwarp implements the Manager API used by infrastructure tools.
package transwarp

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const maxResponseBytes = 16 << 20

var ErrResponse = errors.New("unexpected Manager response")

var ErrPrecheck = errors.New("Manager restart pre-check failed")

var ErrAuthentication = errors.New("Manager authentication failed; check the account and its API login requirements")

type Config struct {
	URL           string
	Username      string
	Password      string
	CAPEM         string
	SkipTLSVerify bool
}

type Client struct {
	base     *url.URL
	http     *http.Client
	username string
	password string
	mu       sync.Mutex
	loggedIn bool
}

func New(cfg Config) (*Client, error) {
	base, err := url.Parse(strings.TrimRight(strings.TrimSpace(cfg.URL), "/"))
	if err != nil || base.Scheme != "https" || base.Hostname() == "" || base.User != nil || base.RawQuery != "" || base.Fragment != "" || (base.Path != "" && base.Path != "/") {
		return nil, errors.New("Manager URL must be an HTTPS origin")
	}
	if cfg.Username == "" || cfg.Password == "" {
		return nil, errors.New("Manager username and password are required")
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	// Certificate verification is bypassed only for a connection whose owner
	// explicitly enabled this setting; other Manager clients keep verifying.
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: cfg.SkipTLSVerify}
	if !cfg.SkipTLSVerify && strings.TrimSpace(cfg.CAPEM) != "" {
		roots, err := x509.SystemCertPool()
		if err != nil {
			return nil, fmt.Errorf("load system certificates: %w", err)
		}
		if !roots.AppendCertsFromPEM([]byte(cfg.CAPEM)) {
			return nil, errors.New("Manager CA certificate is not valid PEM")
		}
		transport.TLSClientConfig.RootCAs = roots
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	return &Client{base: base, username: cfg.Username, password: cfg.Password, http: &http.Client{
		Transport: transport, Jar: jar, Timeout: 35 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("Manager redirects are not followed") },
	}}, nil
}

// Login uses the documented API login and verifies the resulting session.
// Interactive CAPTCHA or two-step requirements remain enforced by Manager.
func (c *Client) Login(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.loggedIn {
		return nil
	}
	if err := c.request(ctx, http.MethodPost, "/api/users/login", nil, map[string]string{"userName": c.username, "userPassword": c.password}, nil); err != nil {
		return err
	}
	var profile struct {
		Username string `json:"userName"`
	}
	if err := c.request(ctx, http.MethodGet, "/api/users/profile", nil, nil, &profile); err != nil {
		return err
	}
	if profile.Username != c.username {
		return ErrAuthentication
	}
	c.loggedIn = true
	return nil
}

func (c *Client) request(ctx context.Context, method, path string, query url.Values, body, out any) error {
	endpoint := *c.base
	endpoint.Path, endpoint.RawQuery = path, query.Encode()
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint.String(), reader)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for _, cookie := range c.http.Jar.Cookies(&endpoint) {
		if cookie.Name == "XSRF-TOKEN" {
			req.Header.Set("X-XSRF-TOKEN", cookie.Value)
		}
	}
	response, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("Manager request failed: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return ErrAuthentication
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("Manager returned HTTP %d", response.StatusCode)
	}
	if out == nil {
		_, err = io.Copy(io.Discard, io.LimitReader(response.Body, maxResponseBytes))
		return err
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return err
	}
	if len(data) > maxResponseBytes {
		return fmt.Errorf("%w: Manager response exceeds the size limit", ErrResponse)
	}
	if target, ok := out.(*managerText); ok {
		*target = managerText(decodeManagerText(data))
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		var mismatch *json.UnmarshalTypeError
		if errors.As(err, &mismatch) {
			return fmt.Errorf("%w: Manager response for %s %s has an incompatible field %q (expected %s, got %s)", ErrResponse, method, path, mismatch.Field, mismatch.Type, mismatch.Value)
		}
		if !json.Valid(data) {
			return fmt.Errorf("%w: Manager returned invalid JSON for %s %s", ErrResponse, method, path)
		}
		return fmt.Errorf("%w: Manager response for %s %s has incompatible fields", ErrResponse, method, path)
	}
	return nil
}

func (c *Client) get(ctx context.Context, path string, query url.Values, out any) error {
	if err := c.Login(ctx); err != nil {
		return err
	}
	err := c.request(ctx, http.MethodGet, path, query, nil, out)
	if errors.Is(err, ErrAuthentication) {
		c.mu.Lock()
		c.loggedIn = false
		c.mu.Unlock()
	}
	return err
}

type Cluster struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}
type Node struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	IP          string `json:"ip"`
	ClusterID   int64  `json:"cluster_id"`
	ClusterName string `json:"cluster_name"`
	Status      string `json:"status"`
	Cores       int    `json:"cores"`
	Memory      int64  `json:"memory"`
	Disk        int64  `json:"disk"`
}
type rawNode struct {
	ID          int64  `json:"id"`
	HostName    string `json:"hostName"`
	IP          string `json:"ipAddress"`
	ClusterID   int64  `json:"clusterId"`
	ClusterName string `json:"clusterName"`
	Status      string `json:"status"`
	Cores       int    `json:"logicalCoreCount"`
	Memory      int64  `json:"memTotal"`
	Disk        int64  `json:"diskTotal"`
}

func (n rawNode) project() Node {
	return Node{n.ID, n.HostName, n.IP, n.ClusterID, n.ClusterName, n.Status, n.Cores, n.Memory, n.Disk}
}

type Product struct {
	Type           string `json:"type"`
	Name           string `json:"name"`
	Version        string `json:"version"`
	DisplayVersion string `json:"display_version"`
}

type Service struct {
	ID                   int64    `json:"id"`
	Name                 string   `json:"name"`
	Type                 string   `json:"type"`
	TypeName             string   `json:"type_name"`
	Version              string   `json:"version"`
	ClusterID            int64    `json:"cluster_id"`
	ClusterName          string   `json:"cluster_name"`
	Health               string   `json:"health"`
	Busy                 bool     `json:"busy"`
	Installed            bool     `json:"installed"`
	Product              *Product `json:"product"`
	Dependencies         []int64  `json:"dependencies"`
	ExternalDependencies []string `json:"external_dependencies"`
}

type rawService struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	Type           string `json:"type"`
	TypeName       string `json:"typeFriendlyName"`
	Version        string `json:"version"`
	DisplayVersion string `json:"displayVersion"`
	ClusterID      int64  `json:"clusterId"`
	ClusterName    string `json:"clusterName"`
	Health         string `json:"health"`
	Busy           bool   `json:"busy"`
	Installed      bool   `json:"installed"`
	ProductInfo    *struct {
		Type           string `json:"type"`
		Name           string `json:"friendlyName"`
		Version        string `json:"version"`
		DisplayVersion string `json:"displayVersion"`
	} `json:"productInfo"`
	Dependencies         []int64  `json:"dependencies"`
	ExternalDependencies []string `json:"externalDependencies"`
}

func (s rawService) project() Service {
	var product *Product
	if p := s.ProductInfo; p != nil {
		product = &Product{Type: p.Type, Name: p.Name, Version: p.Version, DisplayVersion: p.DisplayVersion}
	}
	dependencies := s.Dependencies
	if dependencies == nil {
		dependencies = []int64{}
	}
	external := s.ExternalDependencies
	if external == nil {
		external = []string{}
	}
	version := s.DisplayVersion
	if version == "" {
		version = s.Version
	}
	health := s.Health
	if health == "" {
		health = "UNKNOWN"
	}
	return Service{ID: s.ID, Name: s.Name, Type: s.Type, TypeName: s.TypeName, Version: version, ClusterID: s.ClusterID, ClusterName: s.ClusterName, Health: health, Busy: s.Busy, Installed: s.Installed, Product: product, Dependencies: dependencies, ExternalDependencies: external}
}

type ServiceRelations struct {
	Dependencies []Service `json:"dependencies"`
	Dependents   []Service `json:"dependents"`
}

type Role struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	ServiceID   int64  `json:"service_id"`
	ServiceName string `json:"service_name"`
	Node        Node   `json:"node"`
	Health      string `json:"health"`
	Status      string `json:"status"`
	Installed   bool   `json:"installed"`
}

// Manager returns frontendStatus as an enum name in its OpenAPI contract and
// as {name, title} in deployed responses. Keep the API projection a name.
type managerStatus string

func (s *managerStatus) UnmarshalJSON(data []byte) error {
	var name string
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		*s = "UNKNOWN"
		return nil
	}
	if err := json.Unmarshal(data, &name); err != nil {
		var value struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		name = value.Name
	}
	if name == "" {
		name = "UNKNOWN"
	}
	*s = managerStatus(name)
	return nil
}

type rawRole struct {
	ID          int64         `json:"id"`
	Name        string        `json:"name"`
	Type        string        `json:"roleType"`
	ServiceID   int64         `json:"serviceId"`
	ServiceName string        `json:"serviceName"`
	Node        rawNode       `json:"node"`
	Health      string        `json:"health"`
	Status      managerStatus `json:"frontendStatus"`
	Installed   bool          `json:"installed"`
}

func (r rawRole) project() Role {
	health := r.Health
	if health == "" {
		health = "UNKNOWN"
	}
	status := string(r.Status)
	if status == "" {
		status = "UNKNOWN"
	}
	return Role{r.ID, r.Name, r.Type, r.ServiceID, r.ServiceName, r.Node.project(), health, status, r.Installed}
}

type Metric struct {
	Name      string `json:"name"`
	Value     string `json:"value"`
	Unit      string `json:"unit"`
	Timestamp int64  `json:"timestamp"`
}
type LogEntry struct {
	Hostname    string `json:"hostname"`
	ServiceID   int64  `json:"service_id"`
	ServiceName string `json:"service_name"`
	Time        string `json:"time"`
	Level       string `json:"level"`
	Message     string `json:"message"`
}
type Job struct {
	ID          int64  `json:"id"`
	Status      string `json:"status"`
	Description string `json:"description"`
}

func (c *Client) Clusters(ctx context.Context) ([]Cluster, error) {
	rows := []Cluster{}
	err := c.get(ctx, "/api/clusters", nil, &rows)
	return rows, err
}
func (c *Client) Nodes(ctx context.Context, clusterID int64) ([]Node, error) {
	var rows []rawNode
	err := c.get(ctx, "/api/nodes", url.Values{"clusterId": {strconv.FormatInt(clusterID, 10)}}, &rows)
	result := make([]Node, 0, len(rows))
	for _, n := range rows {
		result = append(result, n.project())
	}
	return result, err
}
func (c *Client) services(ctx context.Context, path string, query url.Values) ([]Service, error) {
	var rows []rawService
	err := c.get(ctx, path, query, &rows)
	result := make([]Service, 0, len(rows))
	for _, row := range rows {
		result = append(result, row.project())
	}
	return result, err
}
func (c *Client) Services(ctx context.Context, clusterID int64) ([]Service, error) {
	return c.services(ctx, "/api/services", url.Values{"clusterId": {strconv.FormatInt(clusterID, 10)}})
}
func (c *Client) Relations(ctx context.Context, serviceID int64) (ServiceRelations, error) {
	dependencies, err := c.services(ctx, fmt.Sprintf("/api/services/%d/dependencies", serviceID), nil)
	if err != nil {
		return ServiceRelations{}, err
	}
	dependents, err := c.services(ctx, fmt.Sprintf("/api/services/%d/dependents", serviceID), nil)
	return ServiceRelations{Dependencies: dependencies, Dependents: dependents}, err
}
func (c *Client) Roles(ctx context.Context, serviceID int64) ([]Role, error) {
	var rows []rawRole
	err := c.get(ctx, "/api/serviceRoles", url.Values{"serviceId": {strconv.FormatInt(serviceID, 10)}}, &rows)
	result := make([]Role, 0, len(rows))
	for _, r := range rows {
		result = append(result, r.project())
	}
	return result, err
}
func (c *Client) Role(ctx context.Context, id int64) (Role, error) {
	var row rawRole
	err := c.get(ctx, fmt.Sprintf("/api/serviceRoles/%d", id), nil, &row)
	return row.project(), err
}
func (c *Client) NodeMetrics(ctx context.Context, nodeID int64, names string) ([]Metric, error) {
	rows := []Metric{}
	err := c.get(ctx, fmt.Sprintf("/api/nodes/%d/metric", nodeID), url.Values{"metricnames": {names}}, &rows)
	return rows, err
}
func (c *Client) SearchLogs(ctx context.Context, serviceID int64, host, level, keywords string, start, end int64) ([]LogEntry, error) {
	var rows []struct {
		Hostname    string `json:"hostname"`
		ServiceID   int64  `json:"serviceId"`
		ServiceName string `json:"serviceName"`
		Time        string `json:"time"`
		Level       string `json:"level"`
		Message     string `json:"message"`
	}
	err := c.get(ctx, "/api/log/search", url.Values{"serviceIds": {strconv.FormatInt(serviceID, 10)}, "hosts": {host}, "level": {level}, "keywords": {keywords}, "start": {strconv.FormatInt(start, 10)}, "end": {strconv.FormatInt(end, 10)}, "timeout": {"20"}}, &rows)
	result := make([]LogEntry, 0, len(rows))
	for _, l := range rows {
		result = append(result, LogEntry{l.Hostname, l.ServiceID, l.ServiceName, l.Time, l.Level, l.Message})
	}
	return result, err
}
func (c *Client) RestartRole(ctx context.Context, roleID int64) (Job, error) {
	var check struct {
		Stoppable bool   `json:"stoppable"`
		Message   string `json:"message"`
	}
	if err := c.get(ctx, "/api/serviceRoles/operations/canStop", url.Values{"roleIds": {strconv.FormatInt(roleID, 10)}}, &check); err != nil {
		return Job{}, fmt.Errorf("%w: %v", ErrPrecheck, err)
	}
	if !check.Stoppable {
		return Job{}, fmt.Errorf("%w: Manager does not allow this role to stop: %s", ErrPrecheck, check.Message)
	}
	var job Job
	err := c.request(ctx, http.MethodPost, "/api/serviceRoles/operations/restart", nil, []int64{roleID}, &job)
	if err == nil && job.ID <= 0 {
		err = errors.New("Manager accepted the request without a job ID; check Manager before submitting again")
	}
	return job, err
}
func (c *Client) Job(ctx context.Context, id int64) (Job, error) {
	var row Job
	err := c.get(ctx, fmt.Sprintf("/api/operations/jobs/%d", id), nil, &row)
	return row, err
}
