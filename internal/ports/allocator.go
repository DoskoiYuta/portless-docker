package ports

import (
	"fmt"
	"net"
)

const (
	// MinPort is the minimum host port to allocate.
	MinPort = 40000
	// MaxPort is the maximum host port to allocate.
	MaxPort = 49999
)

// Allocator handles dynamic port allocation.
type Allocator struct {
	used map[int]bool
}

// NewAllocator creates a new port allocator with optional pre-allocated ports.
func NewAllocator(usedPorts []int) *Allocator {
	used := make(map[int]bool, len(usedPorts))
	for _, p := range usedPorts {
		used[p] = true
	}
	return &Allocator{used: used}
}

// Allocate finds an available port in the configured range.
// It binds to the port to verify it is free, then releases it.
func (a *Allocator) Allocate() (int, error) {
	for port := MinPort; port <= MaxPort; port++ {
		if a.used[port] {
			continue
		}
		if isPortAvailable(port) {
			a.used[port] = true
			return port, nil
		}
	}
	return 0, fmt.Errorf("no available ports in range %d-%d", MinPort, MaxPort)
}

// isPortAvailable checks if a TCP port is available by binding to it.
func isPortAvailable(port int) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return false
	}
	ln.Close()
	return true
}
