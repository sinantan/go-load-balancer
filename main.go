package main

import (
	"context"
	"flag"
	"fmt"
	"go-load-balancer/balancer"
	"go-load-balancer/proxy"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

type Config struct {
	Port                string
	Backends            []string
	Algorithm           string
	HealthCheckInterval time.Duration
	HealthCheckTimeout  time.Duration
}

func main() {
	// Parse command line flags
	config := parseFlags()

	// Validate configuration
	if err := validateConfig(config); err != nil {
		log.Fatalf("Configuration error: %v", err)
	}

	// Create load balancer based on algorithm
	loadBalancer, err := createLoadBalancer(config.Algorithm)
	if err != nil {
		log.Fatalf("Error creating load balancer: %v", err)
	}

	// Add backends to load balancer
	for _, backendURL := range config.Backends {
		parsedURL, err := url.Parse(backendURL)
		if err != nil {
			log.Fatalf("Invalid backend URL %s: %v", backendURL, err)
		}

		backend := &balancer.Backend{
			URL:   parsedURL,
			Alive: true, // Will be checked by health checker
		}

		loadBalancer.AddBackend(backend)
		log.Printf("Added backend: %s", backendURL)
	}

	// Create health checker
	healthChecker := balancer.NewHealthChecker(
		loadBalancer,
		config.HealthCheckInterval,
		config.HealthCheckTimeout,
	)

	// Start health checking
	healthChecker.StartHealthCheck()
	defer healthChecker.StopHealthCheck()

	// Create reverse proxy
	reverseProxy := proxy.NewReverseProxy(loadBalancer, healthChecker)

	// Create HTTP server
	server := &http.Server{
		Addr:         ":" + config.Port,
		Handler:      reverseProxy,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Start server in goroutine
	go func() {
		log.Printf("Load balancer starting on port %s", config.Port)
		log.Printf("Algorithm: %s", config.Algorithm)
		log.Printf("Backends: %v", config.Backends)
		log.Printf("Health check interval: %v", config.HealthCheckInterval)

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	// Handle graceful shutdown
	handleGracefulShutdown(server, healthChecker)
}

// parseFlags parses command line flags and returns configuration
func parseFlags() *Config {
	var (
		port           = flag.String("port", "8080", "Port to listen on")
		backends       = flag.String("backends", "", "Comma-separated list of backend URLs (e.g., http://localhost:3001,http://localhost:3002)")
		algorithm      = flag.String("algorithm", "round-robin", "Load balancing algorithm (round-robin, least-connections, ip-hash)")
		healthInterval = flag.Duration("health-interval", 30*time.Second, "Health check interval")
		healthTimeout  = flag.Duration("health-timeout", 5*time.Second, "Health check timeout")
		showHelp       = flag.Bool("help", false, "Show help message")
	)

	flag.Parse()

	if *showHelp {
		printHelp()
		os.Exit(0)
	}

	var backendList []string
	if *backends != "" {
		backendList = strings.Split(*backends, ",")
		for i, backend := range backendList {
			backendList[i] = strings.TrimSpace(backend)
		}
	}

	return &Config{
		Port:                *port,
		Backends:            backendList,
		Algorithm:           *algorithm,
		HealthCheckInterval: *healthInterval,
		HealthCheckTimeout:  *healthTimeout,
	}
}

// validateConfig validates the configuration
func validateConfig(config *Config) error {
	if len(config.Backends) == 0 {
		return fmt.Errorf("at least one backend must be specified")
	}

	validAlgorithms := map[string]bool{
		"round-robin":       true,
		"least-connections": true,
		"ip-hash":           true,
	}

	if !validAlgorithms[config.Algorithm] {
		return fmt.Errorf("invalid algorithm: %s. Valid options: round-robin, least-connections, ip-hash", config.Algorithm)
	}

	if config.HealthCheckInterval <= 0 {
		return fmt.Errorf("health check interval must be positive")
	}

	if config.HealthCheckTimeout <= 0 {
		return fmt.Errorf("health check timeout must be positive")
	}

	return nil
}

// createLoadBalancer creates a load balancer based on the specified algorithm
func createLoadBalancer(algorithm string) (balancer.LoadBalancer, error) {
	switch algorithm {
	case "round-robin":
		return balancer.NewRoundRobinBalancer(), nil
	case "least-connections":
		return balancer.NewLeastConnectionsBalancer(), nil
	case "ip-hash":
		return balancer.NewIPHashBalancer(), nil
	default:
		return nil, fmt.Errorf("unsupported load balancing algorithm: %s", algorithm)
	}
}

// handleGracefulShutdown handles graceful shutdown on OS signals
func handleGracefulShutdown(server *http.Server, healthChecker balancer.HealthChecker) {
	// Channel to receive OS signals
	sigChan := make(chan os.Signal, 1)

	// Register channel to receive specific signals
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Wait for signal
	sig := <-sigChan
	log.Printf("Received signal: %v. Starting graceful shutdown...", sig)

	// Create context with timeout for graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Stop health checker
	log.Println("Stopping health checker...")
	healthChecker.StopHealthCheck()

	// Shutdown HTTP server
	log.Println("Shutting down HTTP server...")
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Error during server shutdown: %v", err)
		return
	}

	log.Println("Graceful shutdown completed")
}

// printHelp prints usage information
func printHelp() {
	fmt.Println("Go Load Balancer - HTTP Reverse Proxy Load Balancer")
	fmt.Println()
	fmt.Println("USAGE:")
	fmt.Println("    go-load-balancer [OPTIONS]")
	fmt.Println()
	fmt.Println("OPTIONS:")
	fmt.Println("    -port <port>")
	fmt.Println("        Port to listen on (default: 8080)")
	fmt.Println()
	fmt.Println("    -backends <urls>")
	fmt.Println("        Comma-separated list of backend URLs")
	fmt.Println("        Example: http://localhost:3001,http://localhost:3002")
	fmt.Println()
	fmt.Println("    -algorithm <algorithm>")
	fmt.Println("        Load balancing algorithm (default: round-robin)")
	fmt.Println("        Options: round-robin, least-connections, ip-hash")
	fmt.Println()
	fmt.Println("    -health-interval <duration>")
	fmt.Println("        Health check interval (default: 30s)")
	fmt.Println("        Example: 10s, 1m, 2m30s")
	fmt.Println()
	fmt.Println("    -health-timeout <duration>")
	fmt.Println("        Health check timeout (default: 5s)")
	fmt.Println("        Example: 2s, 10s")
	fmt.Println()
	fmt.Println("    -help")
	fmt.Println("        Show this help message")
	fmt.Println()
	fmt.Println("EXAMPLES:")
	fmt.Println("    # Basic usage with round-robin")
	fmt.Println("    go-load-balancer -backends http://localhost:3001,http://localhost:3002")
	fmt.Println()
	fmt.Println("    # Use least-connections algorithm on port 9000")
	fmt.Println("    go-load-balancer -port 9000 -algorithm least-connections -backends http://localhost:3001,http://localhost:3002")
	fmt.Println()
	fmt.Println("    # Use IP hash with custom health check settings")
	fmt.Println("    go-load-balancer -algorithm ip-hash -health-interval 10s -health-timeout 2s -backends http://localhost:3001,http://localhost:3002")
	fmt.Println()
	fmt.Println("ENDPOINTS:")
	fmt.Println("    GET /health")
	fmt.Println("        Load balancer health check endpoint")
	fmt.Println("        Shows status of all backend servers")
}
