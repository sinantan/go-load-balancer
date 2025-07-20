package proxy

import (
	"context"
	"fmt"
	"go-load-balancer/balancer"
	"io"
	"log"
	"net/http"
	"sync/atomic"
	"time"
)

type ReverseProxy struct {
	loadBalancer  balancer.LoadBalancer
	healthChecker balancer.HealthChecker
}

func NewReverseProxy(lb balancer.LoadBalancer, hc balancer.HealthChecker) *ReverseProxy {
	return &ReverseProxy{
		loadBalancer:  lb,
		healthChecker: hc,
	}
}

// ServeHTTP handles incoming HTTP requests
func (rp *ReverseProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Handle health endpoint
	if r.URL.Path == "/health" {
		rp.handleHealthCheck(w, r)
		return
	}

	// Select backend
	backend := rp.loadBalancer.SelectBackend(r)
	if backend == nil {
		http.Error(w, "No healthy backends available", http.StatusServiceUnavailable)
		log.Printf("No healthy backends available for request: %s %s", r.Method, r.URL.Path)
		return
	}

	// Log the request
	log.Printf("Proxying request %s %s to backend %s", r.Method, r.URL.Path, backend.URL.String())

	// Create a new request to the backend
	targetURL := *backend.URL
	targetURL.Path = r.URL.Path
	targetURL.RawQuery = r.URL.RawQuery

	// Create context with timeout
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	// Create the proxy request
	proxyReq, err := http.NewRequestWithContext(ctx, r.Method, targetURL.String(), r.Body)
	if err != nil {
		http.Error(w, "Error creating proxy request", http.StatusInternalServerError)
		log.Printf("Error creating proxy request: %v", err)
		return
	}

	// Copy headers
	for name, values := range r.Header {
		for _, value := range values {
			proxyReq.Header.Add(name, value)
		}
	}

	// Add X-Forwarded-For header
	if clientIP := getClientIP(r); clientIP != "" {
		proxyReq.Header.Set("X-Forwarded-For", clientIP)
	}

	// Add X-Forwarded-Host header
	proxyReq.Header.Set("X-Forwarded-Host", r.Host)

	// Make the request
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Do(proxyReq)
	if err != nil {
		http.Error(w, "Backend server error", http.StatusBadGateway)
		log.Printf("Backend request failed: %v", err)
		atomic.AddInt32(&backend.ErrorCount, 1)

		// Decrement connection count for least-connections balancer
		if lcb, ok := rp.loadBalancer.(*balancer.LeastConnectionsBalancer); ok {
			lcb.DecrementConnections(backend)
		}
		return
	}
	defer resp.Body.Close()

	// Decrement connection count when request completes
	defer func() {
		if lcb, ok := rp.loadBalancer.(*balancer.LeastConnectionsBalancer); ok {
			lcb.DecrementConnections(backend)
		}
	}()

	// Copy response headers
	for name, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(name, value)
		}
	}

	// Set status code
	w.WriteHeader(resp.StatusCode)

	// Copy response body
	_, err = io.Copy(w, resp.Body)
	if err != nil {
		log.Printf("Error copying response body: %v", err)
		atomic.AddInt32(&backend.ErrorCount, 1)
		return
	}

	// Update success count
	atomic.AddInt32(&backend.SuccessCount, 1)
}

// handleHealthCheck handles health check requests
func (rp *ReverseProxy) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	backends := rp.loadBalancer.GetBackends()
	healthyCount := 0

	type BackendStatus struct {
		URL          string `json:"url"`
		Alive        bool   `json:"alive"`
		Connections  int32  `json:"connections"`
		SuccessCount int32  `json:"success_count"`
		ErrorCount   int32  `json:"error_count"`
	}

	type HealthResponse struct {
		Status          string          `json:"status"`
		HealthyBackends int             `json:"healthy_backends"`
		TotalBackends   int             `json:"total_backends"`
		Backends        []BackendStatus `json:"backends"`
	}

	var backendStatuses []BackendStatus
	for _, backend := range backends {
		if backend.Alive {
			healthyCount++
		}

		backendStatuses = append(backendStatuses, BackendStatus{
			URL:          backend.URL.String(),
			Alive:        backend.Alive,
			Connections:  atomic.LoadInt32(&backend.Connections),
			SuccessCount: atomic.LoadInt32(&backend.SuccessCount),
			ErrorCount:   atomic.LoadInt32(&backend.ErrorCount),
		})
	}

	status := "healthy"
	if healthyCount == 0 {
		status = "unhealthy"
		w.WriteHeader(http.StatusServiceUnavailable)
	} else {
		w.WriteHeader(http.StatusOK)
	}

	response := HealthResponse{
		Status:          status,
		HealthyBackends: healthyCount,
		TotalBackends:   len(backends),
		Backends:        backendStatuses,
	}

	w.Header().Set("Content-Type", "application/json")

	// Simple JSON response without external dependencies
	fmt.Fprintf(w, `{
  "status": "%s",
  "healthy_backends": %d,
  "total_backends": %d,
  "backends": [`,
		response.Status, response.HealthyBackends, response.TotalBackends)

	for i, backend := range backendStatuses {
		if i > 0 {
			fmt.Fprint(w, ",")
		}
		fmt.Fprintf(w, `
    {
      "url": "%s",
      "alive": %t,
      "connections": %d,
      "success_count": %d,
      "error_count": %d
    }`, backend.URL, backend.Alive, backend.Connections, backend.SuccessCount, backend.ErrorCount)
	}

	fmt.Fprint(w, `
  ]
}`)
}

// getClientIP extracts the client IP from the request
func getClientIP(r *http.Request) string {
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		return forwarded
	}

	realIP := r.Header.Get("X-Real-IP")
	if realIP != "" {
		return realIP
	}

	return r.RemoteAddr
}
