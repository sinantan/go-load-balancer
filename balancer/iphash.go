package balancer

import (
	"crypto/md5"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

type IPHashBalancer struct {
	backends []*Backend
	mu       sync.RWMutex
}

func NewIPHashBalancer() *IPHashBalancer {
	return &IPHashBalancer{
		backends: make([]*Backend, 0),
	}
}

func (ihb *IPHashBalancer) SelectBackend(request *http.Request) *Backend {
	ihb.mu.RLock()
	defer ihb.mu.RUnlock()

	if len(ihb.backends) == 0 {
		return nil
	}

	aliveBackends := make([]*Backend, 0)
	for _, backend := range ihb.backends {
		if backend.Alive {
			aliveBackends = append(aliveBackends, backend)
		}
	}

	if len(aliveBackends) == 0 {
		return nil
	}

	clientIP := ihb.getClientIP(request)
	hash := ihb.hashIP(clientIP)
	index := hash % uint32(len(aliveBackends))
	return aliveBackends[index]
}

func (ihb *IPHashBalancer) getClientIP(request *http.Request) string {
	forwarded := request.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		ips := strings.Split(forwarded, ",")
		return strings.TrimSpace(ips[0])
	}

	realIP := request.Header.Get("X-Real-IP")
	if realIP != "" {
		return realIP
	}

	host, _, err := net.SplitHostPort(request.RemoteAddr)
	if err != nil {
		return request.RemoteAddr
	}
	return host
}

func (ihb *IPHashBalancer) hashIP(ip string) uint32 {
	hash := md5.Sum([]byte(ip))
	hashStr := fmt.Sprintf("%x", hash[:4])
	hashInt, err := strconv.ParseInt(hashStr, 16, 64)
	if err != nil {
		return 0
	}
	return uint32(hashInt)
}

func (ihb *IPHashBalancer) AddBackend(backend *Backend) {
	ihb.mu.Lock()
	defer ihb.mu.Unlock()
	ihb.backends = append(ihb.backends, backend)
}

func (ihb *IPHashBalancer) RemoveBackend(backend *Backend) {
	ihb.mu.Lock()
	defer ihb.mu.Unlock()

	for i, b := range ihb.backends {
		if b.URL.String() == backend.URL.String() {
			ihb.backends = append(ihb.backends[:i], ihb.backends[i+1:]...)
			break
		}
	}
}

func (ihb *IPHashBalancer) GetBackends() []*Backend {
	ihb.mu.RLock()
	defer ihb.mu.RUnlock()

	backends := make([]*Backend, len(ihb.backends))
	copy(backends, ihb.backends)
	return backends
}

func (ihb *IPHashBalancer) UpdateBackendStatus(backend *Backend, alive bool) {
	ihb.mu.Lock()
	defer ihb.mu.Unlock()

	for _, b := range ihb.backends {
		if b.URL.String() == backend.URL.String() {
			b.Alive = alive
			break
		}
	}
}
