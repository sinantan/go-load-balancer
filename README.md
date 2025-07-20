# HTTP Load Balancer

A command-line HTTP reverse proxy load balancer written in Go. This project implements multiple load balancing algorithms and demonstrates concurrent request handling using goroutines.

## Features

- Multiple load balancing algorithms (round-robin, least-connections, IP hash)
- Interface-based design for extensible algorithms
- Automatic backend health checking
- Graceful shutdown with signal handling
- Request logging and monitoring
- Context-aware request processing with timeouts
- Concurrent request handling using goroutines
- Built-in health endpoint for monitoring

## Installation

### Requirements
- Go 1.21 or later

### Building from Source

```bash
git clone https://github.com/sinantan/go-load-balancer
cd go-load-balancer
go build -o load-balancer main.go
```

## Usage

### Basic Usage

```bash
./load-balancer -backends http://localhost:3001,http://localhost:3002
```

### Advanced Usage

```bash
# Run with least-connections algorithm on port 9000
./load-balancer \
  -port 9000 \
  -algorithm least-connections \
  -backends http://localhost:3001,http://localhost:3002,http://localhost:3003

# Use IP hash with custom health check settings
./load-balancer \
  -algorithm ip-hash \
  -health-interval 10s \
  -health-timeout 2s \
  -backends http://localhost:3001,http://localhost:3002
```

### Command Line Options

| Flag | Default | Description |
|------|---------|-------------|
| `-port` | 8080 | Port to listen on |
| `-backends` | - | Comma-separated list of backend URLs |
| `-algorithm` | round-robin | Load balancing algorithm |
| `-health-interval` | 30s | Health check interval |
| `-health-timeout` | 5s | Health check timeout |
| `-help` | - | Show help message |

## Load Balancing Algorithms

### Round-Robin
Distributes requests sequentially across all available backend servers.

### Least-Connections
Routes requests to the backend server with the fewest active connections.

### IP Hash
Uses client IP address hashing to ensure session affinity - the same client always connects to the same backend server.

## Project Structure

```
go-load-balancer/
├── balancer/           # Load balancing implementations
│   ├── interfaces.go   # Core interfaces
│   ├── roundrobin.go   # Round-robin algorithm
│   ├── leastconnections.go  # Least-connections algorithm
│   ├── iphash.go       # IP hash algorithm
│   └── health.go       # Health checking system
├── proxy/              # Reverse proxy implementation
│   └── reverseproxy.go
├── examples/           # Example applications
│   └── backend-server/ # Test backend servers
├── main.go            # Main application
├── go.mod
└── README.md
```

## Health Monitoring

The load balancer provides a health endpoint at `/health`:

```bash
curl http://localhost:8080/health
```

Example response:
```json
{
  "status": "healthy",
  "healthy_backends": 2,
  "total_backends": 2,
  "backends": [
    {
      "url": "http://localhost:3001",
      "alive": true,
      "connections": 0,
      "success_count": 15,
      "error_count": 0
    }
  ]
}
```

## Testing

### Setting Up Test Backend Servers

Run simple HTTP servers on different ports:

```bash
# Terminal 1
python3 -m http.server 3001

# Terminal 2  
python3 -m http.server 3002

# Terminal 3 (Load Balancer)
./load-balancer -backends http://localhost:3001,http://localhost:3002
```

### Test the Load Balancer

```bash
# Send test requests
curl http://localhost:8080
curl http://localhost:8080
curl http://localhost:8080

# Check health status
curl http://localhost:8080/health
```

You can also use the included test backend server:

```bash
cd examples/backend-server
go run main.go -port 3001 -name "Backend-1"
```

## Development

### Adding New Algorithms

To add a new load balancing algorithm:

1. Create a new file in the `balancer/` directory
2. Implement the `LoadBalancer` interface
3. Add the algorithm to the `createLoadBalancer` function in `main.go`

### Architecture

The application uses an interface-based design that allows for easy extension. The main components are:

- **LoadBalancer Interface**: Defines the contract for load balancing algorithms
- **HealthChecker**: Monitors backend server availability
- **ReverseProxy**: Handles HTTP request forwarding
- **Backend**: Represents individual backend servers with connection tracking

## Configuration

All configuration is handled through command-line flags. The application validates configuration on startup and provides helpful error messages for invalid settings.


## License

This project is open source and available under the MIT License.
