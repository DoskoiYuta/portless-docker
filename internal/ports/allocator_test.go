package ports

import (
	"testing"
)

func TestAllocator_Allocate(t *testing.T) {
	a := NewAllocator(nil)

	port, err := a.Allocate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if port < MinPort || port > MaxPort {
		t.Errorf("port %d out of range [%d, %d]", port, MinPort, MaxPort)
	}

	// Second allocation should return a different port.
	port2, err := a.Allocate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if port2 == port {
		t.Errorf("expected different port, got same: %d", port2)
	}
}

func TestAllocator_WithUsedPorts(t *testing.T) {
	usedPorts := []int{40000, 40001, 40002}
	a := NewAllocator(usedPorts)

	port, err := a.Allocate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if port == 40000 || port == 40001 || port == 40002 {
		t.Errorf("allocated port %d that should be marked as used", port)
	}
}
