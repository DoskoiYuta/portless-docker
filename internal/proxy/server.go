package proxy

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/DoskoiYuta/portless-docker/internal/state"
)

const (
	// SelfCheckInterval はプロキシが残存ルートを確認する間隔。
	SelfCheckInterval = 30 * time.Second
)

// Server は portless-docker のリバースプロキシサーバー。
type Server struct {
	port         int
	stateManager *state.Manager
	httpServer   *http.Server
}

// NewServer は新しいプロキシサーバーを作成する。
func NewServer(port int, sm *state.Manager) *Server {
	handler := NewHandler(sm)
	return &Server{
		port:         port,
		stateManager: sm,
		httpServer: &http.Server{
			Addr:         fmt.Sprintf(":%d", port),
			Handler:      handler,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 60 * time.Second,
			IdleTimeout:  120 * time.Second,
		},
	}
}

// Run はプロキシサーバーを起動し、シャットダウンまでブロックする。
func (s *Server) Run() error {
	// シグナルハンドリングを設定する。
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// セルフチェック用のゴルーチンを起動する。
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.selfCheck(ctx)

	// サーバーを起動する。
	errCh := make(chan error, 1)
	go func() {
		log.Printf("プロキシが :%d でリッスン中", s.port)
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	// シグナルまたはエラーを待機する。
	select {
	case sig := <-sigCh:
		log.Printf("シグナル %v を受信、シャットダウンします", sig)
	case err := <-errCh:
		return fmt.Errorf("サーバーエラー: %w", err)
	}

	// グレースフルシャットダウン。
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	return s.httpServer.Shutdown(shutdownCtx)
}

// Shutdown はサーバーをグレースフルに停止する。
func (s *Server) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return s.httpServer.Shutdown(ctx)
}

// selfCheck は定期的に残存ルートがあるか確認する。
// ルートが残っていない場合、サーバーをシャットダウンする。
func (s *Server) selfCheck(ctx context.Context) {
	ticker := time.NewTicker(SelfCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			has, err := s.stateManager.HasRoutes()
			if err != nil {
				log.Printf("セルフチェックエラー: %v", err)
				continue
			}
			if !has {
				log.Println("残存ルートなし。プロキシをシャットダウンします。")
				_ = s.Shutdown()
				return
			}
		}
	}
}
