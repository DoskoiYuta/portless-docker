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

// Route は単一のサービスルートエントリを表す。
type Route struct {
	Hostname      string    `json:"hostname"`
	HostPort      int       `json:"hostPort"`
	ContainerPort int       `json:"containerPort"`
	Service       string    `json:"service"`
	Type          string    `json:"type"` // "http" または "tcp"
	Directory     string    `json:"directory"`
	ComposeFile   string    `json:"composeFile"`
	OverridePath  string    `json:"overridePath"`
	Detached      bool      `json:"detached"`
	RegisteredAt  time.Time `json:"registeredAt"`
}

// State は portless-docker のグローバル状態を表す。
type State struct {
	ProxyPort int     `json:"proxyPort"`
	Routes    []Route `json:"routes"`
}

// Manager は状態ファイルの操作を管理する。
type Manager struct {
	baseDir string
	lock    *FileLock
}

// NewManager は状態マネージャーを作成し、状態ディレクトリの存在を保証する。
func NewManager() (*Manager, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("ホームディレクトリの取得に失敗: %w", err)
	}

	baseDir := filepath.Join(home, stateDirName)
	return NewManagerWithDir(baseDir)
}

// NewManagerWithDir はカスタムベースディレクトリで状態マネージャーを作成する。
func NewManagerWithDir(baseDir string) (*Manager, error) {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("状態ディレクトリの作成に失敗: %w", err)
	}

	return &Manager{
		baseDir: baseDir,
		lock:    NewFileLock(baseDir),
	}, nil
}

// BaseDir は状態ベースディレクトリのパスを返す。
func (m *Manager) BaseDir() string {
	return m.baseDir
}

// StatePath は state.json のパスを返す。
func (m *Manager) StatePath() string {
	return filepath.Join(m.baseDir, stateFileName)
}

// PIDPath は proxy.pid のパスを返す。
func (m *Manager) PIDPath() string {
	return filepath.Join(m.baseDir, pidFileName)
}

// PortPath は proxy.port のパスを返す。
func (m *Manager) PortPath() string {
	return filepath.Join(m.baseDir, portFileName)
}

// LogPath は proxy.log のパスを返す。
func (m *Manager) LogPath() string {
	return filepath.Join(m.baseDir, logFileName)
}

// Load はディスクから現在の状態を読み込む。
// ファイルが存在しない場合は新しい空の状態を返す。
func (m *Manager) Load() (*State, error) {
	if err := m.lock.Lock(); err != nil {
		return nil, fmt.Errorf("ロック取得に失敗: %w", err)
	}
	defer func() { _ = m.lock.Unlock() }()

	return m.loadUnlocked()
}

// loadUnlocked はロックを取得せずに状態を読み込む（呼び出し元がロックを保持していること）。
func (m *Manager) loadUnlocked() (*State, error) {
	data, err := os.ReadFile(m.StatePath())
	if err != nil {
		if os.IsNotExist(err) {
			return &State{ProxyPort: 1355}, nil
		}
		return nil, fmt.Errorf("状態の読み込みに失敗: %w", err)
	}

	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("状態のパースに失敗: %w", err)
	}
	return &s, nil
}

// Save は状態をディスクに書き込む。
func (m *Manager) Save(s *State) error {
	if err := m.lock.Lock(); err != nil {
		return fmt.Errorf("ロック取得に失敗: %w", err)
	}
	defer func() { _ = m.lock.Unlock() }()

	return m.saveUnlocked(s)
}

// saveUnlocked はロックを取得せずに状態を書き込む（呼び出し元がロックを保持していること）。
func (m *Manager) saveUnlocked(s *State) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("状態のマーシャルに失敗: %w", err)
	}
	return os.WriteFile(m.StatePath(), data, 0644)
}

// WithLock は状態ロックを保持した状態で fn を実行する。
// 関数は現在の状態を受け取り、変更可能。変更された状態は自動的に保存される。
func (m *Manager) WithLock(fn func(s *State) error) error {
	if err := m.lock.Lock(); err != nil {
		return fmt.Errorf("ロック取得に失敗: %w", err)
	}
	defer func() { _ = m.lock.Unlock() }()

	s, err := m.loadUnlocked()
	if err != nil {
		return err
	}

	if err := fn(s); err != nil {
		return err
	}

	return m.saveUnlocked(s)
}
