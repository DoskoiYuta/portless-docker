package proxy

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DoskoiYuta/portless-docker/internal/state"
)

func setupTestHandler(t *testing.T) (*Handler, *state.Manager) {
	t.Helper()
	dir := t.TempDir()
	sm, err := state.NewManagerWithDir(dir)
	if err != nil {
		t.Fatalf("状態マネージャーの作成に失敗: %v", err)
	}
	return NewHandler(sm), sm
}

func TestHandler_MissingHost(t *testing.T) {
	h, _ := setupTestHandler(t)

	req := httptest.NewRequest("GET", "/", nil)
	req.Host = ""
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("ステータス 400 を期待したが %d を取得", w.Code)
	}
}

func TestHandler_NoRoute(t *testing.T) {
	h, _ := setupTestHandler(t)

	req := httptest.NewRequest("GET", "/", nil)
	req.Host = "unknown.localhost:1355"
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("ステータス 404 を期待したが %d を取得", w.Code)
	}
}

func TestHandler_RouteMatch(t *testing.T) {
	h, sm := setupTestHandler(t)

	// テスト用バックエンドを作成する。
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer backend.Close()

	// バックエンドのポートを取得する。
	var backendPort int
	fmt.Sscanf(backend.Listener.Addr().String(), "127.0.0.1:%d", &backendPort)

	// ルートを登録する。
	sm.RegisterRoutes([]state.Route{
		{
			Hostname:  "test.localhost",
			HostPort:  backendPort,
			Service:   "test",
			Directory: "/test",
		},
	})

	req := httptest.NewRequest("GET", "/", nil)
	req.Host = "test.localhost:1355"
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("ステータス 200 を期待したが %d を取得", w.Code)
	}
}
