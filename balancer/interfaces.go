package balancer

import (
	"net/http"
	"net/url"
)

// Backend represents a backend server
type Backend struct {
	URL          *url.URL
	Alive        bool
	Connections  int32
	SuccessCount int32
	ErrorCount   int32
}

// LoadBalancer defines the interface for load balancing strategies
type LoadBalancer interface {
	// SelectBackend chooses a backend server based on the strategy
	SelectBackend(request *http.Request) *Backend

	// AddBackend adds a new backend server
	AddBackend(backend *Backend)

	// RemoveBackend removes a backend server
	RemoveBackend(backend *Backend)

	// GetBackends returns all backend servers
	GetBackends() []*Backend

	// UpdateBackendStatus updates the status of a backend
	UpdateBackendStatus(backend *Backend, alive bool)
}

// HealthChecker interface for health checking backends
type HealthChecker interface {
	// CheckHealth performs health check on a backend
	CheckHealth(backend *Backend) bool

	// StartHealthCheck starts periodic health checks
	StartHealthCheck()

	// StopHealthCheck stops health checks
	StopHealthCheck()
}
