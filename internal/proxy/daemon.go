package proxy

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/DoskoiYuta/portless-docker/internal/state"
)

// Daemon はプロキシデーモンのライフサイクルを管理する。
type Daemon struct {
	stateManager *state.Manager
}

// NewDaemon は新しいデーモンマネージャーを作成する。
func NewDaemon(sm *state.Manager) *Daemon {
	return &Daemon{stateManager: sm}
}

// EnsureRunning はプロキシデーモンがまだ起動していない場合に起動する。
func (d *Daemon) EnsureRunning(proxyPort int) error {
	if d.IsRunning() {
		return nil
	}

	return d.Start(proxyPort)
}

// Start はプロキシをバックグラウンドデーモンプロセスとして起動する。
func (d *Daemon) Start(proxyPort int) error {
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("実行ファイルパスの取得に失敗: %w", err)
	}

	logFile, err := os.OpenFile(d.stateManager.LogPath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("ログファイルのオープンに失敗: %w", err)
	}

	cmd := exec.Command(executable, "__proxy", "--port", strconv.Itoa(proxyPort))
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}

	if err := cmd.Start(); err != nil {
		logFile.Close()
		return fmt.Errorf("プロキシデーモンの起動に失敗: %w", err)
	}

	// PIDファイルとポートファイルを書き込む。
	pid := cmd.Process.Pid
	if err := os.WriteFile(d.stateManager.PIDPath(), []byte(strconv.Itoa(pid)), 0644); err != nil {
		return fmt.Errorf("PIDファイルの書き込みに失敗: %w", err)
	}
	if err := os.WriteFile(d.stateManager.PortPath(), []byte(strconv.Itoa(proxyPort)), 0644); err != nil {
		return fmt.Errorf("ポートファイルの書き込みに失敗: %w", err)
	}

	// 親プロセス終了後もデーモンが継続するようプロセスを解放する。
	cmd.Process.Release()
	logFile.Close()

	// プロキシの準備完了を待機する。
	if err := d.waitForReady(proxyPort, 5*time.Second); err != nil {
		return fmt.Errorf("プロキシの起動に失敗しました。%s を確認してください", d.stateManager.LogPath())
	}

	return nil
}

// Stop はプロキシデーモンを終了する。
func (d *Daemon) Stop() error {
	pid, err := d.readPID()
	if err != nil {
		return nil // PIDファイルなし。停止するものがない。
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		d.cleanup()
		return nil
	}

	// グレースフルシャットダウンのため SIGTERM を送信する。
	if err := process.Signal(syscall.SIGTERM); err != nil {
		d.cleanup()
		return nil
	}

	// プロセスの終了を待機する（タイムアウト付き）。
	done := make(chan struct{})
	go func() {
		process.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		process.Kill()
	}

	d.cleanup()
	return nil
}

// IsRunning はプロキシデーモンが稼働中かどうかを確認する。
func (d *Daemon) IsRunning() bool {
	pid, err := d.readPID()
	if err != nil {
		return false
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// シグナル 0 はシグナルを送信せずにプロセスの存在を確認する。
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// GetPort は実行中のプロキシがリッスンしているポートを返す。
func (d *Daemon) GetPort() (int, error) {
	data, err := os.ReadFile(d.stateManager.PortPath())
	if err != nil {
		return 0, fmt.Errorf("プロキシが実行されていません")
	}
	port, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, fmt.Errorf("無効なポートファイル")
	}
	return port, nil
}

// readPID はPIDファイルからPIDを読み取る。
func (d *Daemon) readPID() (int, error) {
	data, err := os.ReadFile(d.stateManager.PIDPath())
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(data)))
}

// cleanup はPIDファイルとポートファイルを削除する。
func (d *Daemon) cleanup() {
	os.Remove(d.stateManager.PIDPath())
	os.Remove(d.stateManager.PortPath())
}

// waitForReady はプロキシが接続を受け付けるまでポーリングする。
func (d *Daemon) waitForReady(port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	return fmt.Errorf("プロキシが %v 経過後も準備完了しません", timeout)
}
