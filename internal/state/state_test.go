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

	// 空の状態を読み込む。
	s, err := m.Load()
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if s.ProxyPort != 1355 {
		t.Errorf("デフォルトプロキシポート 1355 を期待したが %d を取得", s.ProxyPort)
	}
	if len(s.Routes) != 0 {
		t.Errorf("ルート 0件を期待したが %d 件を取得", len(s.Routes))
	}

	// 保存して再読み込みする。
	s.Routes = []Route{{Hostname: "test.localhost", HostPort: 40001}}
	if err := m.Save(s); err != nil {
		t.Fatalf("保存エラー: %v", err)
	}

	s2, err := m.Load()
	if err != nil {
		t.Fatalf("再読み込みエラー: %v", err)
	}
	if len(s2.Routes) != 1 {
		t.Fatalf("1ルートを期待したが %d を取得", len(s2.Routes))
	}
	if s2.Routes[0].Hostname != "test.localhost" {
		t.Errorf("test.localhost を期待したが %s を取得", s2.Routes[0].Hostname)
	}
}

func TestManager_RegisterRoutes(t *testing.T) {
	m := setupTestManager(t)

	routes := []Route{
		{Hostname: "frontend.localhost", HostPort: 40001, Service: "frontend", Directory: "/project-a"},
		{Hostname: "api.localhost", HostPort: 40002, Service: "api", Directory: "/project-a"},
	}

	if err := m.RegisterRoutes(routes); err != nil {
		t.Fatalf("登録エラー: %v", err)
	}

	all, err := m.GetAllRoutes()
	if err != nil {
		t.Fatalf("全ルート取得エラー: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("2ルートを期待したが %d を取得", len(all))
	}
}

func TestManager_HostnameConflict(t *testing.T) {
	m := setupTestManager(t)

	routes1 := []Route{
		{Hostname: "frontend.localhost", HostPort: 40001, Directory: "/project-a"},
	}
	if err := m.RegisterRoutes(routes1); err != nil {
		t.Fatalf("登録エラー: %v", err)
	}

	routes2 := []Route{
		{Hostname: "frontend.localhost", HostPort: 40002, Directory: "/project-b"},
	}
	err := m.RegisterRoutes(routes2)
	if err == nil {
		t.Fatal("ホスト名競合エラーを期待")
	}
}

func TestManager_OverwriteSameDirectory(t *testing.T) {
	m := setupTestManager(t)

	routes1 := []Route{
		{Hostname: "frontend.localhost", HostPort: 40001, Directory: "/project-a"},
	}
	if err := m.RegisterRoutes(routes1); err != nil {
		t.Fatalf("登録エラー: %v", err)
	}

	routes2 := []Route{
		{Hostname: "frontend.localhost", HostPort: 40099, Directory: "/project-a"},
	}
	if err := m.RegisterRoutes(routes2); err != nil {
		t.Fatalf("上書きエラー: %v", err)
	}

	all, err := m.GetAllRoutes()
	if err != nil {
		t.Fatalf("全ルート取得エラー: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("1ルートを期待したが %d を取得", len(all))
	}
	if all[0].HostPort != 40099 {
		t.Errorf("ポート 40099 を期待したが %d を取得", all[0].HostPort)
	}
}

func TestManager_UnregisterRoutes(t *testing.T) {
	m := setupTestManager(t)

	routes := []Route{
		{Hostname: "frontend.localhost", HostPort: 40001, Directory: "/project-a"},
		{Hostname: "api.localhost", HostPort: 40002, Directory: "/project-a"},
	}
	_ = m.RegisterRoutes(routes)

	removed, err := m.UnregisterRoutes("/project-a")
	if err != nil {
		t.Fatalf("登録解除エラー: %v", err)
	}
	if len(removed) != 2 {
		t.Errorf("2件の削除を期待したが %d を取得", len(removed))
	}

	has, _ := m.HasRoutes()
	if has {
		t.Error("ルートが残っていないことを期待")
	}
}

func TestFileLock(t *testing.T) {
	dir := t.TempDir()
	lockDir := filepath.Join(dir, lockDirName)

	fl := NewFileLock(dir)

	if err := fl.Lock(); err != nil {
		t.Fatalf("ロックエラー: %v", err)
	}

	if _, err := os.Stat(lockDir); os.IsNotExist(err) {
		t.Error("ロックディレクトリが存在するべき")
	}

	if err := fl.Unlock(); err != nil {
		t.Fatalf("アンロックエラー: %v", err)
	}

	if _, err := os.Stat(lockDir); !os.IsNotExist(err) {
		t.Error("ロックディレクトリが削除されているべき")
	}
}
