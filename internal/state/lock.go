package state

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	lockDirName   = "state.lock"
	lockTimeout   = 5 * time.Second
	lockRetry     = 50 * time.Millisecond
	staleDuration = 10 * time.Second
)

// FileLock は mkdir ベースのファイルロックを提供する。
type FileLock struct {
	dir string
}

// NewFileLock は指定ベースディレクトリ内に新しいファイルロックを作成する。
func NewFileLock(baseDir string) *FileLock {
	return &FileLock{
		dir: filepath.Join(baseDir, lockDirName),
	}
}

// Lock はタイムアウトと失効検出付きでファイルロックを取得する。
func (fl *FileLock) Lock() error {
	deadline := time.Now().Add(lockTimeout)

	for {
		err := os.Mkdir(fl.dir, 0755)
		if err == nil {
			// 失効検出用のタイムスタンプファイルを書き込む。
			ts := filepath.Join(fl.dir, "ts")
			_ = os.WriteFile(ts, []byte(time.Now().Format(time.RFC3339Nano)), 0644)
			return nil
		}

		if !os.IsExist(err) {
			return fmt.Errorf("ロックの作成に失敗: %w", err)
		}

		// ロックが失効していないか確認する。
		if fl.isStale() {
			_ = os.RemoveAll(fl.dir)
			continue
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("ロック取得が %v 後にタイムアウト", lockTimeout)
		}

		time.Sleep(lockRetry)
	}
}

// Unlock はファイルロックを解放する。
func (fl *FileLock) Unlock() error {
	return os.RemoveAll(fl.dir)
}

// isStale はロックが staleDuration より古いかどうかを確認する。
func (fl *FileLock) isStale() bool {
	ts := filepath.Join(fl.dir, "ts")
	data, err := os.ReadFile(ts)
	if err != nil {
		// タイムスタンプが読めない場合、ディレクトリの更新時刻を確認する。
		info, err := os.Stat(fl.dir)
		if err != nil {
			return false
		}
		return time.Since(info.ModTime()) > staleDuration
	}

	t, err := time.Parse(time.RFC3339Nano, string(data))
	if err != nil {
		return true
	}

	return time.Since(t) > staleDuration
}
