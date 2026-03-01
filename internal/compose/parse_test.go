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
				t.Fatal("PortMapping が nil でないことを期待")
			}
			if pm.HostPort != tt.wantHost {
				t.Errorf("HostPort = %d, 期待値 %d", pm.HostPort, tt.wantHost)
			}
			if pm.ContainerPort != tt.wantCont {
				t.Errorf("ContainerPort = %d, 期待値 %d", pm.ContainerPort, tt.wantCont)
			}
			if pm.Protocol != tt.wantProto {
				t.Errorf("Protocol = %q, 期待値 %q", pm.Protocol, tt.wantProto)
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
				t.Errorf("ServiceSubdomain(%q) = %q, 期待値 %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestFindComposeFile(t *testing.T) {
	dir := t.TempDir()

	// Composeファイルが存在しない場合
	_, err := FindComposeFile(dir, "")
	if err == nil {
		t.Error("Composeファイルが存在しない場合にエラーを期待")
	}

	// docker-compose.yml を作成
	p := filepath.Join(dir, "docker-compose.yml")
	os.WriteFile(p, []byte("services: {}"), 0644)

	found, err := FindComposeFile(dir, "")
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if filepath.Base(found) != "docker-compose.yml" {
		t.Errorf("docker-compose.yml を期待したが %s を取得", filepath.Base(found))
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
		t.Fatalf("予期しないエラー: %v", err)
	}

	if len(cf.Services) != 2 {
		t.Fatalf("2サービスを期待したが %d を取得", len(cf.Services))
	}

	fe, ok := cf.Services["frontend"]
	if !ok {
		t.Fatal("frontend サービスを期待")
	}
	if fe.ContainerPort != 3000 {
		t.Errorf("frontend ContainerPort = %d, 期待値 3000", fe.ContainerPort)
	}
	if fe.Type != ServiceTypeHTTP {
		t.Errorf("frontend Type = %q, 期待値 %q", fe.Type, ServiceTypeHTTP)
	}

	api, ok := cf.Services["api"]
	if !ok {
		t.Fatal("api サービスを期待")
	}
	if api.ContainerPort != 8080 {
		t.Errorf("api ContainerPort = %d, 期待値 8080", api.ContainerPort)
	}
	if api.Type != ServiceTypeHTTP {
		t.Errorf("api Type = %q, 期待値 %q", api.Type, ServiceTypeHTTP)
	}
}

func TestParseWithTCPServices(t *testing.T) {
	dir := t.TempDir()
	composePath := filepath.Join(dir, "docker-compose.yml")

	content := `services:
  api:
    ports:
      - "8080:8080"
  postgres:
    image: postgres
    ports:
      - "5432:5432"
  redis:
    image: redis
    ports:
      - "6379:6379"
  mysql:
    image: mysql
    ports:
      - "3306:3306"
`
	os.WriteFile(composePath, []byte(content), 0644)

	cf, err := Parse(composePath, nil)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}

	if len(cf.Services) != 4 {
		t.Fatalf("4サービスを期待したが %d を取得", len(cf.Services))
	}

	tests := []struct {
		name     string
		wantType ServiceType
	}{
		{"api", ServiceTypeHTTP},
		{"postgres", ServiceTypeTCP},
		{"redis", ServiceTypeTCP},
		{"mysql", ServiceTypeTCP},
	}

	for _, tt := range tests {
		svc, ok := cf.Services[tt.name]
		if !ok {
			t.Errorf("%s サービスが見つかりません", tt.name)
			continue
		}
		if svc.Type != tt.wantType {
			t.Errorf("%s Type = %q, 期待値 %q", tt.name, svc.Type, tt.wantType)
		}
	}
}

func TestClassifyPort(t *testing.T) {
	tests := []struct {
		port     int
		wantType ServiceType
	}{
		{80, ServiceTypeHTTP},
		{3000, ServiceTypeHTTP},
		{8080, ServiceTypeHTTP},
		{443, ServiceTypeHTTP},
		{5432, ServiceTypeTCP},
		{3306, ServiceTypeTCP},
		{6379, ServiceTypeTCP},
		{27017, ServiceTypeTCP},
		{9092, ServiceTypeTCP},
	}

	for _, tt := range tests {
		got := ClassifyPort(tt.port)
		if got != tt.wantType {
			t.Errorf("ClassifyPort(%d) = %q, 期待値 %q", tt.port, got, tt.wantType)
		}
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
		t.Fatalf("予期しないエラー: %v", err)
	}

	if len(cf.Services) != 1 {
		t.Fatalf("1サービスを期待したが %d を取得", len(cf.Services))
	}
	if _, ok := cf.Services["frontend"]; !ok {
		t.Error("frontend サービスを期待")
	}
}
