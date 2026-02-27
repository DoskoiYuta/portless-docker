package compose

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// ComposeFile represents a parsed docker-compose.yml.
type ComposeFile struct {
	Path     string
	Services map[string]ServiceDef
}

// ServiceDef represents a service definition in a compose file.
type ServiceDef struct {
	Name          string
	ContainerPort int
	HostPort      int
	RawPorts      []string
}

// PortMapping represents a parsed port mapping.
type PortMapping struct {
	HostPort      int
	ContainerPort int
	Protocol      string
}

// composeFileNames is the ordered list of compose file names to search for.
var composeFileNames = []string{
	"docker-compose.yml",
	"docker-compose.yaml",
	"compose.yml",
	"compose.yaml",
}

// FindComposeFile looks for a compose file in the given directory.
// If filePath is provided, it uses that directly.
func FindComposeFile(dir, filePath string) (string, error) {
	if filePath != "" {
		abs, err := filepath.Abs(filePath)
		if err != nil {
			return "", fmt.Errorf("invalid path: %w", err)
		}
		if _, err := os.Stat(abs); err != nil {
			return "", fmt.Errorf("compose file not found: %s", abs)
		}
		return abs, nil
	}

	for _, name := range composeFileNames {
		p := filepath.Join(dir, name)
		if _, err := os.Stat(p); err == nil {
			abs, _ := filepath.Abs(p)
			return abs, nil
		}
	}

	return "", fmt.Errorf("No docker-compose.yml found.")
}

// Parse reads and parses a docker-compose.yml file, returning services with port mappings.
// ignoredServices is a set of service names to skip.
func Parse(composePath string, ignoredServices map[string]bool) (*ComposeFile, error) {
	data, err := os.ReadFile(composePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read compose file: %w", err)
	}

	var raw struct {
		Services map[string]struct {
			Ports []interface{} `yaml:"ports"`
		} `yaml:"services"`
	}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse compose file: %w", err)
	}

	cf := &ComposeFile{
		Path:     composePath,
		Services: make(map[string]ServiceDef),
	}

	for name, svc := range raw.Services {
		if ignoredServices[name] {
			continue
		}
		if len(svc.Ports) == 0 {
			continue
		}

		var rawPorts []string
		for _, p := range svc.Ports {
			rawPorts = append(rawPorts, fmt.Sprintf("%v", p))
		}

		pm := parseFirstTCPPort(rawPorts)
		if pm == nil {
			continue
		}

		cf.Services[name] = ServiceDef{
			Name:          name,
			ContainerPort: pm.ContainerPort,
			HostPort:      pm.HostPort,
			RawPorts:      rawPorts,
		}
	}

	if len(cf.Services) == 0 {
		return nil, fmt.Errorf("No services with port mappings found.")
	}

	return cf, nil
}

// parseFirstTCPPort extracts the first TCP port mapping from raw port strings.
func parseFirstTCPPort(rawPorts []string) *PortMapping {
	for _, raw := range rawPorts {
		raw = strings.TrimSpace(raw)
		pm := parsePortString(raw)
		if pm != nil && (pm.Protocol == "" || pm.Protocol == "tcp") {
			return pm
		}
	}
	return nil
}

// parsePortString parses a single Docker Compose port string.
// Supported formats:
//   - "3000"
//   - "3000:3000"
//   - "8080:80"
//   - "127.0.0.1:3000:3000"
//   - "0.0.0.0:3000:3000/tcp"
func parsePortString(s string) *PortMapping {
	// Strip protocol suffix.
	proto := ""
	if idx := strings.LastIndex(s, "/"); idx != -1 {
		proto = s[idx+1:]
		s = s[:idx]
	}

	parts := strings.Split(s, ":")

	var hostPort, containerPort int
	var err error

	switch len(parts) {
	case 1:
		// "3000"
		hostPort, err = strconv.Atoi(parts[0])
		if err != nil {
			return nil
		}
		containerPort = hostPort
	case 2:
		// "8080:80"
		hostPort, err = strconv.Atoi(parts[0])
		if err != nil {
			return nil
		}
		containerPort, err = strconv.Atoi(parts[1])
		if err != nil {
			return nil
		}
	case 3:
		// "127.0.0.1:3000:3000" — first part is IP, skip it.
		hostPort, err = strconv.Atoi(parts[1])
		if err != nil {
			return nil
		}
		containerPort, err = strconv.Atoi(parts[2])
		if err != nil {
			return nil
		}
	default:
		return nil
	}

	return &PortMapping{
		HostPort:      hostPort,
		ContainerPort: containerPort,
		Protocol:      proto,
	}
}

// ServiceSubdomain converts a service name to a valid subdomain.
// Underscores become hyphens, uppercase becomes lowercase.
func ServiceSubdomain(name string) string {
	s := strings.ReplaceAll(name, "_", "-")
	s = strings.ToLower(s)
	return s
}
