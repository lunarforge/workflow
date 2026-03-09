package gateway

import (
	"net/http"
	"time"
)

// AdapterOption configures the GatewayAdapter.
type AdapterOption func(*adapterConfig)

type adapterConfig struct {
	gracePeriod       time.Duration
	heartbeatInterval time.Duration
	runCacheSize      int
	apiKey            string
	httpClient        *http.Client
}

func defaultAdapterConfig() adapterConfig {
	return adapterConfig{
		gracePeriod:       30 * time.Second,
		heartbeatInterval: 10 * time.Second,
		runCacheSize:      100_000,
		httpClient:        http.DefaultClient,
	}
}

// WithGracePeriod sets how long a disconnected subsystem remains in the
// registry before being removed. Default: 30s.
func WithGracePeriod(d time.Duration) AdapterOption {
	return func(c *adapterConfig) { c.gracePeriod = d }
}

// WithHeartbeatInterval sets the interval between heartbeat pings sent to
// subsystems. Default: 10s.
func WithHeartbeatInterval(d time.Duration) AdapterOption {
	return func(c *adapterConfig) { c.heartbeatInterval = d }
}

// WithRunCacheSize sets the maximum number of run-to-subsystem mappings
// to cache. Default: 100,000.
func WithRunCacheSize(n int) AdapterOption {
	return func(c *adapterConfig) { c.runCacheSize = n }
}

// WithAPIKey sets the shared secret that subsystems must present to register.
// An empty key disables authentication.
func WithAPIKey(key string) AdapterOption {
	return func(c *adapterConfig) { c.apiKey = key }
}

// WithHTTPClient sets the HTTP client used to create Connect-Go clients for
// proxying requests to subsystems.
func WithHTTPClient(c *http.Client) AdapterOption {
	return func(cfg *adapterConfig) { cfg.httpClient = c }
}

// ClientOption configures the subsystem-side gateway client.
type ClientOption func(*clientConfig)

type clientConfig struct {
	gatewayAddr       string
	apiKey            string
	subsystemID       string
	subsystemName     string
	description       string
	localEndpoint     string
	reconnectInterval time.Duration
	maxReconnect      time.Duration
	httpClient        *http.Client
}

func defaultClientConfig() clientConfig {
	return clientConfig{
		reconnectInterval: 1 * time.Second,
		maxReconnect:      30 * time.Second,
		httpClient:        http.DefaultClient,
	}
}

// WithGatewayAddr sets the gateway server address to connect to.
func WithGatewayAddr(addr string) ClientOption {
	return func(c *clientConfig) { c.gatewayAddr = addr }
}

// WithClientAPIKey sets the shared secret for authentication with the gateway.
func WithClientAPIKey(key string) ClientOption {
	return func(c *clientConfig) { c.apiKey = key }
}

// WithSubsystemID sets the unique subsystem identifier.
func WithSubsystemID(id string) ClientOption {
	return func(c *clientConfig) { c.subsystemID = id }
}

// WithSubsystemName sets the human-readable subsystem name.
func WithSubsystemName(name string) ClientOption {
	return func(c *clientConfig) { c.subsystemName = name }
}

// WithDescription sets the subsystem description.
func WithDescription(desc string) ClientOption {
	return func(c *clientConfig) { c.description = desc }
}

// WithLocalEndpoint sets the FlowWatch HTTP endpoint of this subsystem
// that the gateway will connect back to for proxying requests.
func WithLocalEndpoint(endpoint string) ClientOption {
	return func(c *clientConfig) { c.localEndpoint = endpoint }
}

// WithClientHTTPClient sets the HTTP client for the gateway connection.
func WithClientHTTPClient(c *http.Client) ClientOption {
	return func(cfg *clientConfig) { cfg.httpClient = c }
}
