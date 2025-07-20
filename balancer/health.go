package balancer

import (
	"context"
	"log"
	"net/http"
	"sync/atomic"
	"time"
)

// DefaultHealthChecker implements health checking functionality
type DefaultHealthChecker struct {
	balancer LoadBalancer
	interval time.Duration
	timeout  time.Duration
	ctx      context.Context
	cancel   context.CancelFunc
	running  int32
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(balancer LoadBalancer, interval, timeout time.Duration) *DefaultHealthChecker {
	ctx, cancel := context.WithCancel(context.Background())
	return &DefaultHealthChecker{
		balancer: balancer,
		interval: interval,
		timeout:  timeout,
		ctx:      ctx,
		cancel:   cancel,
	}
}

// CheckHealth performs a health check on a specific backend
func (hc *DefaultHealthChecker) CheckHealth(backend *Backend) bool {
	ctx, cancel := context.WithTimeout(hc.ctx, hc.timeout)
	defer cancel()

	healthURL := backend.URL.String() + "/health"
	req, err := http.NewRequestWithContext(ctx, "GET", healthURL, nil)
	if err != nil {
		log.Printf("Health check error creating request for %s: %v", backend.URL.String(), err)
		return false
	}

	client := &http.Client{Timeout: hc.timeout}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Health check failed for %s: %v", backend.URL.String(), err)
		atomic.AddInt32(&backend.ErrorCount, 1)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		atomic.AddInt32(&backend.SuccessCount, 1)
		log.Printf("Health check passed for %s", backend.URL.String())
		return true
	}

	atomic.AddInt32(&backend.ErrorCount, 1)
	log.Printf("Health check failed for %s with status: %d", backend.URL.String(), resp.StatusCode)
	return false
}

// StartHealthCheck starts periodic health checks
func (hc *DefaultHealthChecker) StartHealthCheck() {
	if !atomic.CompareAndSwapInt32(&hc.running, 0, 1) {
		return // Already running
	}

	log.Printf("Starting health checker with interval: %v", hc.interval)

	go func() {
		defer atomic.StoreInt32(&hc.running, 0)

		ticker := time.NewTicker(hc.interval)
		defer ticker.Stop()

		for {
			select {
			case <-hc.ctx.Done():
				log.Println("Health checker stopped")
				return
			case <-ticker.C:
				hc.performHealthChecks()
			}
		}
	}()
}

// StopHealthCheck stops the health checker
func (hc *DefaultHealthChecker) StopHealthCheck() {
	if atomic.LoadInt32(&hc.running) == 0 {
		return // Not running
	}

	log.Println("Stopping health checker")
	hc.cancel()
}

// performHealthChecks checks all backends
func (hc *DefaultHealthChecker) performHealthChecks() {
	backends := hc.balancer.GetBackends()

	for _, backend := range backends {
		go func(b *Backend) {
			alive := hc.CheckHealth(b)
			previousState := b.Alive
			hc.balancer.UpdateBackendStatus(b, alive)

			if previousState != alive {
				status := "DOWN"
				if alive {
					status = "UP"
				}
				log.Printf("Backend %s status changed to %s", b.URL.String(), status)
			}
		}(backend)
	}
}
