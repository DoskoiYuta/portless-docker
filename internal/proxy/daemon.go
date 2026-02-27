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

// Daemon manages the proxy daemon lifecycle.
type Daemon struct {
	stateManager *state.Manager
}

// NewDaemon creates a new daemon manager.
func NewDaemon(sm *state.Manager) *Daemon {
	return &Daemon{stateManager: sm}
}

// EnsureRunning starts the proxy daemon if it isn't already running.
func (d *Daemon) EnsureRunning(proxyPort int) error {
	if d.IsRunning() {
		return nil
	}

	return d.Start(proxyPort)
}

// Start launches the proxy as a background daemon process.
func (d *Daemon) Start(proxyPort int) error {
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	logFile, err := os.OpenFile(d.stateManager.LogPath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	cmd := exec.Command(executable, "__proxy", "--port", strconv.Itoa(proxyPort))
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}

	if err := cmd.Start(); err != nil {
		logFile.Close()
		return fmt.Errorf("failed to start proxy daemon: %w", err)
	}

	// Write PID and port files.
	pid := cmd.Process.Pid
	if err := os.WriteFile(d.stateManager.PIDPath(), []byte(strconv.Itoa(pid)), 0644); err != nil {
		return fmt.Errorf("failed to write PID file: %w", err)
	}
	if err := os.WriteFile(d.stateManager.PortPath(), []byte(strconv.Itoa(proxyPort)), 0644); err != nil {
		return fmt.Errorf("failed to write port file: %w", err)
	}

	// Release the process so it continues after parent exits.
	cmd.Process.Release()
	logFile.Close()

	// Wait for the proxy to be ready.
	if err := d.waitForReady(proxyPort, 5*time.Second); err != nil {
		return fmt.Errorf("Proxy failed to start. Check %s", d.stateManager.LogPath())
	}

	return nil
}

// Stop terminates the proxy daemon.
func (d *Daemon) Stop() error {
	pid, err := d.readPID()
	if err != nil {
		return nil // No PID file, nothing to stop.
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		d.cleanup()
		return nil
	}

	// Send SIGTERM for graceful shutdown.
	if err := process.Signal(syscall.SIGTERM); err != nil {
		d.cleanup()
		return nil
	}

	// Wait for process to exit (with timeout).
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

// IsRunning checks if the proxy daemon is alive.
func (d *Daemon) IsRunning() bool {
	pid, err := d.readPID()
	if err != nil {
		return false
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// Signal 0 checks if the process exists without actually sending a signal.
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// GetPort returns the port the running proxy is listening on.
func (d *Daemon) GetPort() (int, error) {
	data, err := os.ReadFile(d.stateManager.PortPath())
	if err != nil {
		return 0, fmt.Errorf("proxy not running")
	}
	port, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, fmt.Errorf("invalid port file")
	}
	return port, nil
}

// readPID reads the PID from the PID file.
func (d *Daemon) readPID() (int, error) {
	data, err := os.ReadFile(d.stateManager.PIDPath())
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(data)))
}

// cleanup removes PID and port files.
func (d *Daemon) cleanup() {
	os.Remove(d.stateManager.PIDPath())
	os.Remove(d.stateManager.PortPath())
}

// waitForReady polls until the proxy is accepting connections.
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

	return fmt.Errorf("proxy not ready after %v", timeout)
}
