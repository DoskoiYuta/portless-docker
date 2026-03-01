package ports

import (
	"fmt"
	"net"
)

const (
	// MinPort は割り当て可能なホストポートの最小値。
	MinPort = 40000
	// MaxPort は割り当て可能なホストポートの最大値。
	MaxPort = 49999
)

// Allocator は動的ポート割り当てを処理する。
type Allocator struct {
	used map[int]bool
}

// NewAllocator は新しいポートアロケータを作成する。usedPorts で既に使用中のポートを指定可能。
func NewAllocator(usedPorts []int) *Allocator {
	used := make(map[int]bool, len(usedPorts))
	for _, p := range usedPorts {
		used[p] = true
	}
	return &Allocator{used: used}
}

// Allocate は設定範囲内で利用可能なポートを見つける。
// ポートにバインドして空いていることを確認した後、リリースする。
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
	return 0, fmt.Errorf("範囲 %d-%d で利用可能なポートがありません", MinPort, MaxPort)
}

// isPortAvailable はTCPポートにバインドして利用可能かどうかを確認する。
func isPortAvailable(port int) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return false
	}
	ln.Close()
	return true
}
