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

// FileLock provides mkdir-based file locking.
type FileLock struct {
	dir string
}

// NewFileLock creates a new file lock in the given base directory.
func NewFileLock(baseDir string) *FileLock {
	return &FileLock{
		dir: filepath.Join(baseDir, lockDirName),
	}
}

// Lock acquires the file lock with timeout and stale detection.
func (fl *FileLock) Lock() error {
	deadline := time.Now().Add(lockTimeout)

	for {
		err := os.Mkdir(fl.dir, 0755)
		if err == nil {
			// Write a timestamp file for stale detection.
			ts := filepath.Join(fl.dir, "ts")
			os.WriteFile(ts, []byte(time.Now().Format(time.RFC3339Nano)), 0644)
			return nil
		}

		if !os.IsExist(err) {
			return fmt.Errorf("failed to create lock: %w", err)
		}

		// Check for stale lock.
		if fl.isStale() {
			os.RemoveAll(fl.dir)
			continue
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("lock acquisition timed out after %v", lockTimeout)
		}

		time.Sleep(lockRetry)
	}
}

// Unlock releases the file lock.
func (fl *FileLock) Unlock() error {
	return os.RemoveAll(fl.dir)
}

// isStale checks if the lock is older than staleDuration.
func (fl *FileLock) isStale() bool {
	ts := filepath.Join(fl.dir, "ts")
	data, err := os.ReadFile(ts)
	if err != nil {
		// If we can't read timestamp, check directory mod time.
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
