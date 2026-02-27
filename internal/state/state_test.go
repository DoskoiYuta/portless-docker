package state

import (
	"os"
	"path/filepath"
	"testing"
)

func setupTestManager(t *testing.T) *Manager {
	t.Helper()
	dir := t.TempDir()
	return &Manager{
		baseDir: dir,
		lock:    NewFileLock(dir),
	}
}

func TestManager_LoadSave(t *testing.T) {
	m := setupTestManager(t)

	// Load empty state.
	s, err := m.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.ProxyPort != 1355 {
		t.Errorf("expected default proxy port 1355, got %d", s.ProxyPort)
	}
	if len(s.Routes) != 0 {
		t.Errorf("expected no routes, got %d", len(s.Routes))
	}

	// Save and reload.
	s.Routes = []Route{{Hostname: "test.localhost", HostPort: 40001}}
	if err := m.Save(s); err != nil {
		t.Fatalf("save error: %v", err)
	}

	s2, err := m.Load()
	if err != nil {
		t.Fatalf("reload error: %v", err)
	}
	if len(s2.Routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(s2.Routes))
	}
	if s2.Routes[0].Hostname != "test.localhost" {
		t.Errorf("expected test.localhost, got %s", s2.Routes[0].Hostname)
	}
}

func TestManager_RegisterRoutes(t *testing.T) {
	m := setupTestManager(t)

	routes := []Route{
		{Hostname: "frontend.localhost", HostPort: 40001, Service: "frontend", Directory: "/project-a"},
		{Hostname: "api.localhost", HostPort: 40002, Service: "api", Directory: "/project-a"},
	}

	if err := m.RegisterRoutes(routes); err != nil {
		t.Fatalf("register error: %v", err)
	}

	all, err := m.GetAllRoutes()
	if err != nil {
		t.Fatalf("get all error: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 routes, got %d", len(all))
	}
}

func TestManager_HostnameConflict(t *testing.T) {
	m := setupTestManager(t)

	routes1 := []Route{
		{Hostname: "frontend.localhost", HostPort: 40001, Directory: "/project-a"},
	}
	if err := m.RegisterRoutes(routes1); err != nil {
		t.Fatalf("register error: %v", err)
	}

	routes2 := []Route{
		{Hostname: "frontend.localhost", HostPort: 40002, Directory: "/project-b"},
	}
	err := m.RegisterRoutes(routes2)
	if err == nil {
		t.Fatal("expected hostname conflict error")
	}
}

func TestManager_OverwriteSameDirectory(t *testing.T) {
	m := setupTestManager(t)

	routes1 := []Route{
		{Hostname: "frontend.localhost", HostPort: 40001, Directory: "/project-a"},
	}
	if err := m.RegisterRoutes(routes1); err != nil {
		t.Fatalf("register error: %v", err)
	}

	routes2 := []Route{
		{Hostname: "frontend.localhost", HostPort: 40099, Directory: "/project-a"},
	}
	if err := m.RegisterRoutes(routes2); err != nil {
		t.Fatalf("overwrite error: %v", err)
	}

	all, err := m.GetAllRoutes()
	if err != nil {
		t.Fatalf("get all error: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("expected 1 route, got %d", len(all))
	}
	if all[0].HostPort != 40099 {
		t.Errorf("expected port 40099, got %d", all[0].HostPort)
	}
}

func TestManager_UnregisterRoutes(t *testing.T) {
	m := setupTestManager(t)

	routes := []Route{
		{Hostname: "frontend.localhost", HostPort: 40001, Directory: "/project-a"},
		{Hostname: "api.localhost", HostPort: 40002, Directory: "/project-a"},
	}
	m.RegisterRoutes(routes)

	removed, err := m.UnregisterRoutes("/project-a")
	if err != nil {
		t.Fatalf("unregister error: %v", err)
	}
	if len(removed) != 2 {
		t.Errorf("expected 2 removed, got %d", len(removed))
	}

	has, _ := m.HasRoutes()
	if has {
		t.Error("expected no routes remaining")
	}
}

func TestFileLock(t *testing.T) {
	dir := t.TempDir()
	lockDir := filepath.Join(dir, lockDirName)

	fl := NewFileLock(dir)

	if err := fl.Lock(); err != nil {
		t.Fatalf("lock error: %v", err)
	}

	if _, err := os.Stat(lockDir); os.IsNotExist(err) {
		t.Error("lock directory should exist")
	}

	if err := fl.Unlock(); err != nil {
		t.Fatalf("unlock error: %v", err)
	}

	if _, err := os.Stat(lockDir); !os.IsNotExist(err) {
		t.Error("lock directory should be removed")
	}
}
