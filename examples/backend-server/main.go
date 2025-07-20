package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"
)

// Server represents a simple backend server
type Server struct {
	port      string
	name      string
	startTime time.Time
	requests  int
}

// HealthResponse represents health check response
type HealthResponse struct {
	Status    string    `json:"status"`
	Server    string    `json:"server"`
	Port      string    `json:"port"`
	Uptime    string    `json:"uptime"`
	Requests  int       `json:"requests"`
	Timestamp time.Time `json:"timestamp"`
}

// InfoResponse represents server info response
type InfoResponse struct {
	Server    string            `json:"server"`
	Port      string            `json:"port"`
	Method    string            `json:"method"`
	Path      string            `json:"path"`
	Headers   map[string]string `json:"headers"`
	Query     map[string]string `json:"query"`
	RemoteIP  string            `json:"remote_ip"`
	UserAgent string            `json:"user_agent"`
	Timestamp time.Time         `json:"timestamp"`
}

func main() {
	var (
		port = flag.String("port", "3001", "Port to listen on")
		name = flag.String("name", "", "Server name (default: backend-<port>)")
	)
	flag.Parse()

	if *name == "" {
		*name = fmt.Sprintf("backend-%s", *port)
	}

	server := &Server{
		port:      *port,
		name:      *name,
		startTime: time.Now(),
	}

	// Setup routes
	http.HandleFunc("/", server.handleRoot)
	http.HandleFunc("/health", server.handleHealth)
	http.HandleFunc("/info", server.handleInfo)
	http.HandleFunc("/delay/", server.handleDelay)
	http.HandleFunc("/error/", server.handleError)

	log.Printf("Backend server '%s' starting on port %s", server.name, server.port)
	log.Printf("Health check: http://localhost:%s/health", server.port)
	log.Printf("Server info: http://localhost:%s/info", server.port)
	log.Printf("Delay test: http://localhost:%s/delay/5s", server.port)
	log.Printf("Error test: http://localhost:%s/error/500", server.port)

	if err := http.ListenAndServe(":"+server.port, nil); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}

// handleRoot handles root requests
func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	s.requests++

	response := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <title>Backend Server - %s</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 50px; }
        .server { background: #f0f0f0; padding: 20px; border-radius: 10px; }
        .info { margin: 10px 0; }
    </style>
</head>
<body>
    <div class="server">
        <h1>🖥️ Backend Server</h1>
        <div class="info"><strong>Server:</strong> %s</div>
        <div class="info"><strong>Port:</strong> %s</div>
        <div class="info"><strong>Request #:</strong> %d</div>
        <div class="info"><strong>Time:</strong> %s</div>
        <div class="info"><strong>Path:</strong> %s</div>
        <div class="info"><strong>Method:</strong> %s</div>
        <div class="info"><strong>Client IP:</strong> %s</div>
        <div class="info"><strong>User Agent:</strong> %s</div>
        
        <h3>Available Endpoints:</h3>
        <ul>
            <li><a href="/health">Health Check</a></li>
            <li><a href="/info">Server Info (JSON)</a></li>
            <li><a href="/delay/3s">Delay Test (3 seconds)</a></li>
            <li><a href="/error/500">Error Test (500)</a></li>
        </ul>
    </div>
</body>
</html>`,
		s.name, s.name, s.port, s.requests,
		time.Now().Format("2006-01-02 15:04:05"),
		r.URL.Path, r.Method, r.RemoteAddr, r.UserAgent())

	log.Printf("Request #%d: %s %s from %s", s.requests, r.Method, r.URL.Path, r.RemoteAddr)

	w.Header().Set("Content-Type", "text/html")
	w.Header().Set("X-Backend-Server", s.name)
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, response)
}

// handleHealth handles health check requests
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.requests++

	uptime := time.Since(s.startTime)

	response := HealthResponse{
		Status:    "healthy",
		Server:    s.name,
		Port:      s.port,
		Uptime:    uptime.String(),
		Requests:  s.requests,
		Timestamp: time.Now(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Backend-Server", s.name)
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Error encoding response", http.StatusInternalServerError)
		return
	}

	log.Printf("Health check #%d from %s", s.requests, r.RemoteAddr)
}

// handleInfo handles info requests with detailed request information
func (s *Server) handleInfo(w http.ResponseWriter, r *http.Request) {
	s.requests++

	// Extract headers
	headers := make(map[string]string)
	for name, values := range r.Header {
		if len(values) > 0 {
			headers[name] = values[0]
		}
	}

	// Extract query parameters
	query := make(map[string]string)
	for name, values := range r.URL.Query() {
		if len(values) > 0 {
			query[name] = values[0]
		}
	}

	response := InfoResponse{
		Server:    s.name,
		Port:      s.port,
		Method:    r.Method,
		Path:      r.URL.Path,
		Headers:   headers,
		Query:     query,
		RemoteIP:  r.RemoteAddr,
		UserAgent: r.UserAgent(),
		Timestamp: time.Now(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Backend-Server", s.name)
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Error encoding response", http.StatusInternalServerError)
		return
	}

	log.Printf("Info request #%d: %s %s from %s", s.requests, r.Method, r.URL.Path, r.RemoteAddr)
}

// handleDelay handles delay test requests
func (s *Server) handleDelay(w http.ResponseWriter, r *http.Request) {
	s.requests++

	// Extract delay from path
	delayStr := r.URL.Path[len("/delay/"):]
	delay, err := time.ParseDuration(delayStr)
	if err != nil {
		http.Error(w, "Invalid delay format. Use format like: /delay/5s", http.StatusBadRequest)
		return
	}

	log.Printf("Delay request #%d: %s for %v from %s", s.requests, r.URL.Path, delay, r.RemoteAddr)

	// Sleep for specified duration
	time.Sleep(delay)

	response := fmt.Sprintf(`{
  "server": "%s",
  "port": "%s",
  "message": "Delayed response after %s",
  "timestamp": "%s"
}`, s.name, s.port, delay, time.Now().Format(time.RFC3339))

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Backend-Server", s.name)
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, response)
}

// handleError handles error test requests
func (s *Server) handleError(w http.ResponseWriter, r *http.Request) {
	s.requests++

	// Extract status code from path
	statusStr := r.URL.Path[len("/error/"):]
	statusCode, err := strconv.Atoi(statusStr)
	if err != nil {
		http.Error(w, "Invalid status code. Use format like: /error/500", http.StatusBadRequest)
		return
	}

	log.Printf("Error request #%d: %s status %d from %s", s.requests, r.URL.Path, statusCode, r.RemoteAddr)

	response := fmt.Sprintf(`{
  "server": "%s",
  "port": "%s",
  "error": "Simulated error with status %d",
  "timestamp": "%s"
}`, s.name, s.port, statusCode, time.Now().Format(time.RFC3339))

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Backend-Server", s.name)
	w.WriteHeader(statusCode)
	fmt.Fprint(w, response)
}
