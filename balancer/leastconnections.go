package balancer

import (
	"net/http"
	"sync"
	"sync/atomic"
)

type LeastConnectionsBalancer struct {
	backends []*Backend
	mu       sync.RWMutex
}

func NewLeastConnectionsBalancer() *LeastConnectionsBalancer {
	return &LeastConnectionsBalancer{
		backends: make([]*Backend, 0),
	}
}
func (lcb *LeastConnectionsBalancer) SelectBackend(request *http.Request) *Backend {
	lcb.mu.RLock()
	defer lcb.mu.RUnlock()

	if len(lcb.backends) == 0 {
		return nil
	}

	var selected *Backend
	minConnections := int32(-1)

	for _, backend := range lcb.backends {
		if !backend.Alive {
			continue
		}

		connections := atomic.LoadInt32(&backend.Connections)
		if minConnections == -1 || connections < minConnections {
			minConnections = connections
			selected = backend
		}
	}

	if selected != nil {
		atomic.AddInt32(&selected.Connections, 1)
	}

	return selected
}

func (lcb *LeastConnectionsBalancer) AddBackend(backend *Backend) {
	lcb.mu.Lock()
	defer lcb.mu.Unlock()
	lcb.backends = append(lcb.backends, backend)
}

func (lcb *LeastConnectionsBalancer) RemoveBackend(backend *Backend) {
	lcb.mu.Lock()
	defer lcb.mu.Unlock()

	for i, b := range lcb.backends {
		if b.URL.String() == backend.URL.String() {
			lcb.backends = append(lcb.backends[:i], lcb.backends[i+1:]...)
			break
		}
	}
}

func (lcb *LeastConnectionsBalancer) GetBackends() []*Backend {
	lcb.mu.RLock()
	defer lcb.mu.RUnlock()

	backends := make([]*Backend, len(lcb.backends))
	copy(backends, lcb.backends)
	return backends
}

func (lcb *LeastConnectionsBalancer) UpdateBackendStatus(backend *Backend, alive bool) {
	lcb.mu.Lock()
	defer lcb.mu.Unlock()

	for _, b := range lcb.backends {
		if b.URL.String() == backend.URL.String() {
			b.Alive = alive
			break
		}
	}
}
func (lcb *LeastConnectionsBalancer) DecrementConnections(backend *Backend) {
	atomic.AddInt32(&backend.Connections, -1)
}
