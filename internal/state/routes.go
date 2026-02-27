package state

import (
	"fmt"
	"time"
)

// RegisterRoutes はプロジェクトのルートを追加する。同じディレクトリのルートが既に存在する場合は
// 置き換えられる。異なるディレクトリとのホスト名競合が検出された場合はエラーを返す。
func (m *Manager) RegisterRoutes(routes []Route) error {
	return m.WithLock(func(s *State) error {
		if len(routes) == 0 {
			return nil
		}
		dir := routes[0].Directory

		// 異なるディレクトリとのホスト名競合を確認する。
		for _, newRoute := range routes {
			for _, existing := range s.Routes {
				if existing.Hostname == newRoute.Hostname && existing.Directory != dir {
					return fmt.Errorf(
						"%q は既に %s によって登録されています",
						newRoute.Hostname, existing.Directory,
					)
				}
			}
		}

		// このディレクトリの既存ルートを削除する（上書き）。
		filtered := make([]Route, 0, len(s.Routes))
		for _, r := range s.Routes {
			if r.Directory != dir {
				filtered = append(filtered, r)
			}
		}

		// 新しいルートを追加する。
		now := time.Now().UTC()
		for i := range routes {
			routes[i].RegisteredAt = now
		}
		s.Routes = append(filtered, routes...)
		return nil
	})
}

// UnregisterRoutes は指定ディレクトリの全ルートを削除する。
// 削除されたルートを返す。
func (m *Manager) UnregisterRoutes(directory string) ([]Route, error) {
	var removed []Route
	err := m.WithLock(func(s *State) error {
		filtered := make([]Route, 0, len(s.Routes))
		for _, r := range s.Routes {
			if r.Directory == directory {
				removed = append(removed, r)
			} else {
				filtered = append(filtered, r)
			}
		}
		s.Routes = filtered
		return nil
	})
	return removed, err
}

// UnregisterAllRoutes は全ルートを削除して返す。
func (m *Manager) UnregisterAllRoutes() ([]Route, error) {
	var removed []Route
	err := m.WithLock(func(s *State) error {
		removed = s.Routes
		s.Routes = nil
		return nil
	})
	return removed, err
}

// GetRoutes は指定ディレクトリの全ルートを返す。
func (m *Manager) GetRoutes(directory string) ([]Route, error) {
	s, err := m.Load()
	if err != nil {
		return nil, err
	}

	var routes []Route
	for _, r := range s.Routes {
		if r.Directory == directory {
			routes = append(routes, r)
		}
	}
	return routes, nil
}

// GetAllRoutes は登録済みの全ルートを返す。
func (m *Manager) GetAllRoutes() ([]Route, error) {
	s, err := m.Load()
	if err != nil {
		return nil, err
	}
	return s.Routes, nil
}

// GetOverridePath は指定ディレクトリのオーバーライドパスを返す（存在する場合）。
func (m *Manager) GetOverridePath(directory string) (string, error) {
	routes, err := m.GetRoutes(directory)
	if err != nil {
		return "", err
	}
	if len(routes) > 0 {
		return routes[0].OverridePath, nil
	}
	return "", nil
}

// HasRoutes は登録済みルートが存在するかどうかを確認する。
func (m *Manager) HasRoutes() (bool, error) {
	s, err := m.Load()
	if err != nil {
		return false, err
	}
	return len(s.Routes) > 0, nil
}

// RouteCount は登録済みルートの数を返す。
func (m *Manager) RouteCount() (int, error) {
	s, err := m.Load()
	if err != nil {
		return 0, err
	}
	return len(s.Routes), nil
}

// GetUsedPorts は現在割り当て済みの全ホストポートを返す。
func (m *Manager) GetUsedPorts() ([]int, error) {
	s, err := m.Load()
	if err != nil {
		return nil, err
	}
	var ports []int
	for _, r := range s.Routes {
		ports = append(ports, r.HostPort)
	}
	return ports, nil
}
