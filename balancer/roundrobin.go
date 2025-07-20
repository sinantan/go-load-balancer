package balancer

import (
	"net/http"
	"sync"
	"sync/atomic"
)

type RoundRobinBalancer struct {
	backends []*Backend
	current  uint64
	mu       sync.RWMutex
}

func NewRoundRobinBalancer() *RoundRobinBalancer {
	return &RoundRobinBalancer{
		backends: make([]*Backend, 0),
	}
}

func (rb *RoundRobinBalancer) SelectBackend(request *http.Request) *Backend {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	if len(rb.backends) == 0 {
		return nil
	}

	aliveBackends := make([]*Backend, 0)
	for _, backend := range rb.backends {
		if backend.Alive {
			aliveBackends = append(aliveBackends, backend)
		}
	}

	if len(aliveBackends) == 0 {
		return nil
	}

	index := atomic.AddUint64(&rb.current, 1) % uint64(len(aliveBackends))
	return aliveBackends[index]
}

func (rb *RoundRobinBalancer) AddBackend(backend *Backend) {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	rb.backends = append(rb.backends, backend)
}

func (rb *RoundRobinBalancer) RemoveBackend(backend *Backend) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	for i, b := range rb.backends {
		if b.URL.String() == backend.URL.String() {
			rb.backends = append(rb.backends[:i], rb.backends[i+1:]...)
			break
		}
	}
}

func (rb *RoundRobinBalancer) GetBackends() []*Backend {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	backends := make([]*Backend, len(rb.backends))
	copy(backends, rb.backends)
	return backends
}

func (rb *RoundRobinBalancer) UpdateBackendStatus(backend *Backend, alive bool) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	for _, b := range rb.backends {
		if b.URL.String() == backend.URL.String() {
			b.Alive = alive
			break
		}
	}
}
