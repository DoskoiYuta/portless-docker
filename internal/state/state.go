package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	stateDirName  = ".portless-docker"
	stateFileName = "state.json"
	pidFileName   = "proxy.pid"
	portFileName  = "proxy.port"
	logFileName   = "proxy.log"
)

// Route represents a single service route entry.
type Route struct {
	Hostname      string    `json:"hostname"`
	HostPort      int       `json:"hostPort"`
	ContainerPort int       `json:"containerPort"`
	Service       string    `json:"service"`
	Directory     string    `json:"directory"`
	ComposeFile   string    `json:"composeFile"`
	OverridePath  string    `json:"overridePath"`
	Detached      bool      `json:"detached"`
	RegisteredAt  time.Time `json:"registeredAt"`
}

// State represents the global portless-docker state.
type State struct {
	ProxyPort int     `json:"proxyPort"`
	Routes    []Route `json:"routes"`
}

// Manager handles state file operations.
type Manager struct {
	baseDir string
	lock    *FileLock
}

// NewManager creates a state manager, ensuring the state directory exists.
func NewManager() (*Manager, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	baseDir := filepath.Join(home, stateDirName)
	return NewManagerWithDir(baseDir)
}

// NewManagerWithDir creates a state manager with a custom base directory.
func NewManagerWithDir(baseDir string) (*Manager, error) {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create state directory: %w", err)
	}

	return &Manager{
		baseDir: baseDir,
		lock:    NewFileLock(baseDir),
	}, nil
}

// BaseDir returns the state base directory path.
func (m *Manager) BaseDir() string {
	return m.baseDir
}

// StatePath returns the path to state.json.
func (m *Manager) StatePath() string {
	return filepath.Join(m.baseDir, stateFileName)
}

// PIDPath returns the path to proxy.pid.
func (m *Manager) PIDPath() string {
	return filepath.Join(m.baseDir, pidFileName)
}

// PortPath returns the path to proxy.port.
func (m *Manager) PortPath() string {
	return filepath.Join(m.baseDir, portFileName)
}

// LogPath returns the path to proxy.log.
func (m *Manager) LogPath() string {
	return filepath.Join(m.baseDir, logFileName)
}

// Load reads the current state from disk.
// Returns a new empty state if the file doesn't exist.
func (m *Manager) Load() (*State, error) {
	if err := m.lock.Lock(); err != nil {
		return nil, fmt.Errorf("failed to acquire lock: %w", err)
	}
	defer m.lock.Unlock()

	return m.loadUnlocked()
}

// loadUnlocked reads state without acquiring the lock (caller must hold it).
func (m *Manager) loadUnlocked() (*State, error) {
	data, err := os.ReadFile(m.StatePath())
	if err != nil {
		if os.IsNotExist(err) {
			return &State{ProxyPort: 1355}, nil
		}
		return nil, fmt.Errorf("failed to read state: %w", err)
	}

	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("failed to parse state: %w", err)
	}
	return &s, nil
}

// Save writes the state to disk.
func (m *Manager) Save(s *State) error {
	if err := m.lock.Lock(); err != nil {
		return fmt.Errorf("failed to acquire lock: %w", err)
	}
	defer m.lock.Unlock()

	return m.saveUnlocked(s)
}

// saveUnlocked writes state without acquiring the lock (caller must hold it).
func (m *Manager) saveUnlocked(s *State) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}
	return os.WriteFile(m.StatePath(), data, 0644)
}

// WithLock executes fn while holding the state lock.
// The function receives the current state and can modify it; the modified state is saved.
func (m *Manager) WithLock(fn func(s *State) error) error {
	if err := m.lock.Lock(); err != nil {
		return fmt.Errorf("failed to acquire lock: %w", err)
	}
	defer m.lock.Unlock()

	s, err := m.loadUnlocked()
	if err != nil {
		return err
	}

	if err := fn(s); err != nil {
		return err
	}

	return m.saveUnlocked(s)
}
