package state

import (
	"fmt"
	"time"
)

// RegisterRoutes adds routes for a project. If the same directory already has routes,
// they are replaced. Returns an error if a hostname conflict is detected with a different directory.
func (m *Manager) RegisterRoutes(routes []Route) error {
	return m.WithLock(func(s *State) error {
		if len(routes) == 0 {
			return nil
		}
		dir := routes[0].Directory

		// Check for hostname conflicts with different directories.
		for _, newRoute := range routes {
			for _, existing := range s.Routes {
				if existing.Hostname == newRoute.Hostname && existing.Directory != dir {
					return fmt.Errorf(
						"%q is already registered by %s",
						newRoute.Hostname, existing.Directory,
					)
				}
			}
		}

		// Remove existing routes for this directory (overwrite).
		filtered := make([]Route, 0, len(s.Routes))
		for _, r := range s.Routes {
			if r.Directory != dir {
				filtered = append(filtered, r)
			}
		}

		// Add new routes.
		now := time.Now().UTC()
		for i := range routes {
			routes[i].RegisteredAt = now
		}
		s.Routes = append(filtered, routes...)
		return nil
	})
}

// UnregisterRoutes removes all routes for the given directory.
// Returns the removed routes.
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

// UnregisterAllRoutes removes all routes and returns them.
func (m *Manager) UnregisterAllRoutes() ([]Route, error) {
	var removed []Route
	err := m.WithLock(func(s *State) error {
		removed = s.Routes
		s.Routes = nil
		return nil
	})
	return removed, err
}

// GetRoutes returns all routes for the given directory.
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

// GetAllRoutes returns all registered routes.
func (m *Manager) GetAllRoutes() ([]Route, error) {
	s, err := m.Load()
	if err != nil {
		return nil, err
	}
	return s.Routes, nil
}

// GetOverridePath returns the override path for the given directory, if any.
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

// HasRoutes checks if there are any registered routes.
func (m *Manager) HasRoutes() (bool, error) {
	s, err := m.Load()
	if err != nil {
		return false, err
	}
	return len(s.Routes) > 0, nil
}

// RouteCount returns the number of registered routes.
func (m *Manager) RouteCount() (int, error) {
	s, err := m.Load()
	if err != nil {
		return 0, err
	}
	return len(s.Routes), nil
}

// GetUsedPorts returns all currently allocated host ports.
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
