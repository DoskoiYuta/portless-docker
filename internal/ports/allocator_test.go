package ports

import (
	"testing"
)

func TestAllocator_Allocate(t *testing.T) {
	a := NewAllocator(nil)

	port, err := a.Allocate()
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if port < MinPort || port > MaxPort {
		t.Errorf("ポート %d が範囲 [%d, %d] 外", port, MinPort, MaxPort)
	}

	// 2回目の割り当ては異なるポートを返すべき。
	port2, err := a.Allocate()
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if port2 == port {
		t.Errorf("異なるポートを期待したが同じポートを取得: %d", port2)
	}
}

func TestAllocator_WithUsedPorts(t *testing.T) {
	usedPorts := []int{40000, 40001, 40002}
	a := NewAllocator(usedPorts)

	port, err := a.Allocate()
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if port == 40000 || port == 40001 || port == 40002 {
		t.Errorf("使用済みとしてマークされたポート %d が割り当てられた", port)
	}
}

func TestAllocator_AllocateDeterministic(t *testing.T) {
	a1 := NewAllocator(nil)
	a2 := NewAllocator(nil)

	key := "myproject:postgres:5432"

	port1, err := a1.AllocateDeterministic(key)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if port1 < MinPort || port1 > MaxPort {
		t.Errorf("ポート %d が範囲 [%d, %d] 外", port1, MinPort, MaxPort)
	}

	port2, err := a2.AllocateDeterministic(key)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}

	// 同じキーなら同じポートが返されるべき。
	if port1 != port2 {
		t.Errorf("同じキーで異なるポート: %d != %d", port1, port2)
	}
}

func TestAllocator_DeterministicDifferentKeys(t *testing.T) {
	a := NewAllocator(nil)

	port1, err := a.AllocateDeterministic("project-a:postgres:5432")
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}

	port2, err := a.AllocateDeterministic("project-b:postgres:5432")
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}

	// 異なるキーなら異なるポートが返されるべき。
	if port1 == port2 {
		t.Errorf("異なるキーで同じポート: %d", port1)
	}
}
