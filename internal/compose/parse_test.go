package compose

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParsePortString(t *testing.T) {
	tests := []struct {
		input     string
		wantHost  int
		wantCont  int
		wantProto string
	}{
		{"3000:3000", 3000, 3000, ""},
		{"8080:80", 8080, 80, ""},
		{"3000", 3000, 3000, ""},
		{"127.0.0.1:3000:3000", 3000, 3000, ""},
		{"0.0.0.0:3000:3000/tcp", 3000, 3000, "tcp"},
		{"9090:9090/udp", 9090, 9090, "udp"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			pm := parsePortString(tt.input)
			if pm == nil {
				t.Fatal("expected non-nil PortMapping")
			}
			if pm.HostPort != tt.wantHost {
				t.Errorf("HostPort = %d, want %d", pm.HostPort, tt.wantHost)
			}
			if pm.ContainerPort != tt.wantCont {
				t.Errorf("ContainerPort = %d, want %d", pm.ContainerPort, tt.wantCont)
			}
			if pm.Protocol != tt.wantProto {
				t.Errorf("Protocol = %q, want %q", pm.Protocol, tt.wantProto)
			}
		})
	}
}

func TestServiceSubdomain(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"frontend", "frontend"},
		{"api", "api"},
		{"my-service", "my-service"},
		{"web_app", "web-app"},
		{"My_Service", "my-service"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ServiceSubdomain(tt.input)
			if got != tt.want {
				t.Errorf("ServiceSubdomain(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestFindComposeFile(t *testing.T) {
	dir := t.TempDir()

	// No compose file found
	_, err := FindComposeFile(dir, "")
	if err == nil {
		t.Error("expected error when no compose file exists")
	}

	// Create docker-compose.yml
	p := filepath.Join(dir, "docker-compose.yml")
	os.WriteFile(p, []byte("services: {}"), 0644)

	found, err := FindComposeFile(dir, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if filepath.Base(found) != "docker-compose.yml" {
		t.Errorf("expected docker-compose.yml, got %s", filepath.Base(found))
	}
}

func TestParse(t *testing.T) {
	dir := t.TempDir()
	composePath := filepath.Join(dir, "docker-compose.yml")

	content := `services:
  frontend:
    build: ./frontend
    ports:
      - "3000:3000"
  api:
    build: ./api
    ports:
      - "8080:8080"
  redis:
    image: redis
`
	os.WriteFile(composePath, []byte(content), 0644)

	cf, err := Parse(composePath, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cf.Services) != 2 {
		t.Fatalf("expected 2 services, got %d", len(cf.Services))
	}

	fe, ok := cf.Services["frontend"]
	if !ok {
		t.Fatal("expected frontend service")
	}
	if fe.ContainerPort != 3000 {
		t.Errorf("frontend ContainerPort = %d, want 3000", fe.ContainerPort)
	}

	api, ok := cf.Services["api"]
	if !ok {
		t.Fatal("expected api service")
	}
	if api.ContainerPort != 8080 {
		t.Errorf("api ContainerPort = %d, want 8080", api.ContainerPort)
	}
}

func TestParseWithIgnore(t *testing.T) {
	dir := t.TempDir()
	composePath := filepath.Join(dir, "docker-compose.yml")

	content := `services:
  frontend:
    ports:
      - "3000:3000"
  api:
    ports:
      - "8080:8080"
`
	os.WriteFile(composePath, []byte(content), 0644)

	ignored := map[string]bool{"api": true}
	cf, err := Parse(composePath, ignored)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cf.Services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(cf.Services))
	}
	if _, ok := cf.Services["frontend"]; !ok {
		t.Error("expected frontend service")
	}
}
