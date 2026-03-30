package compose

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractEnvLabels_Map(t *testing.T) {
	labels := map[string]interface{}{
		"portless-docker.env.API_URL": "http://{{api.host}}:{{api.port}}/v1",
		"portless-docker.env.WS_URL":  "ws://{{api.host}}:{{api.port}}/ws",
		"com.example.unrelated":       "ignored",
		"portless-docker.env.":        "empty key should be skipped",
	}

	result := extractEnvLabels(labels)

	if len(result) != 2 {
		t.Fatalf("2エントリを期待したが %d を取得", len(result))
	}
	if result["API_URL"] != "http://{{api.host}}:{{api.port}}/v1" {
		t.Errorf("API_URL = %q", result["API_URL"])
	}
	if result["WS_URL"] != "ws://{{api.host}}:{{api.port}}/ws" {
		t.Errorf("WS_URL = %q", result["WS_URL"])
	}
}

func TestExtractEnvLabels_List(t *testing.T) {
	labels := []interface{}{
		"portless-docker.env.API_URL={{api.url}}",
		"portless-docker.env.DB_PORT={{postgres.port}}",
		"com.example.unrelated=ignored",
	}

	result := extractEnvLabels(labels)

	if len(result) != 2 {
		t.Fatalf("2エントリを期待したが %d を取得", len(result))
	}
	if result["API_URL"] != "{{api.url}}" {
		t.Errorf("API_URL = %q", result["API_URL"])
	}
	if result["DB_PORT"] != "{{postgres.port}}" {
		t.Errorf("DB_PORT = %q", result["DB_PORT"])
	}
}

func TestExtractEnvLabels_Nil(t *testing.T) {
	result := extractEnvLabels(nil)
	if len(result) != 0 {
		t.Errorf("空の結果を期待したが %d エントリを取得", len(result))
	}
}

func TestResolveEnvTemplate(t *testing.T) {
	endpoints := map[string]ServiceEndpoint{
		"api": {
			Hostname: "api.myproject.localtest.me",
			Port:     1355,
			Type:     ServiceTypeHTTP,
		},
		"postgres": {
			Hostname: "localhost",
			Port:     40123,
			Type:     ServiceTypeTCP,
		},
	}

	tests := []struct {
		name      string
		tmpl      string
		proxyPort int
		want      string
		wantErr   bool
	}{
		{
			name:      "HTTP url",
			tmpl:      "{{api.url}}",
			proxyPort: 1355,
			want:      "http://api.myproject.localtest.me:1355",
		},
		{
			name:      "HTTP host and port",
			tmpl:      "http://{{api.host}}:{{api.port}}/v1",
			proxyPort: 1355,
			want:      "http://api.myproject.localtest.me:1355/v1",
		},
		{
			name:      "TCP url",
			tmpl:      "{{postgres.url}}",
			proxyPort: 1355,
			want:      "localhost:40123",
		},
		{
			name:      "TCP port only",
			tmpl:      "{{postgres.port}}",
			proxyPort: 1355,
			want:      "40123",
		},
		{
			name:      "mixed services",
			tmpl:      "http://{{api.host}}:{{api.port}}?db_port={{postgres.port}}",
			proxyPort: 1355,
			want:      "http://api.myproject.localtest.me:1355?db_port=40123",
		},
		{
			name:      "no placeholders",
			tmpl:      "http://localhost:3000",
			proxyPort: 1355,
			want:      "http://localhost:3000",
		},
		{
			name:      "unknown service",
			tmpl:      "{{unknown.url}}",
			proxyPort: 1355,
			wantErr:   true,
		},
		{
			name:      "unknown property",
			tmpl:      "{{api.unknown}}",
			proxyPort: 1355,
			wantErr:   true,
		},
		{
			name:      "proxy port",
			tmpl:      "http://{{api.host}}:{{proxy.port}}/v1",
			proxyPort: 1355,
			want:      "http://api.myproject.localtest.me:1355/v1",
		},
		{
			name:      "proxy port with custom port",
			tmpl:      "http://{{api.host}}:{{proxy.port}}/v1",
			proxyPort: 9999,
			want:      "http://api.myproject.localtest.me:9999/v1",
		},
		{
			name:      "proxy port only",
			tmpl:      "{{proxy.port}}",
			proxyPort: 2000,
			want:      "2000",
		},
		{
			name:      "proxy unsupported property",
			tmpl:      "{{proxy.host}}",
			proxyPort: 1355,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveEnvTemplate(tt.tmpl, endpoints, tt.proxyPort)
			if tt.wantErr {
				if err == nil {
					t.Errorf("エラーを期待したがnil")
				}
				return
			}
			if err != nil {
				t.Fatalf("予期しないエラー: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveAllEnvLabels(t *testing.T) {
	labels := map[string]map[string]string{
		"frontend": {
			"API_URL": "http://{{api.host}}:{{api.port}}/v1",
			"WS_URL":  "ws://{{api.host}}:{{api.port}}/ws",
		},
		"worker": {
			"DB_PORT": "{{postgres.port}}",
		},
	}

	endpoints := map[string]ServiceEndpoint{
		"api": {
			Hostname: "api.myproject.localtest.me",
			Port:     1355,
			Type:     ServiceTypeHTTP,
		},
		"postgres": {
			Hostname: "localhost",
			Port:     40123,
			Type:     ServiceTypeTCP,
		},
	}

	result, err := ResolveAllEnvLabels(labels, endpoints, 1355)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("2サービスを期待したが %d を取得", len(result))
	}

	if result["frontend"]["API_URL"] != "http://api.myproject.localtest.me:1355/v1" {
		t.Errorf("frontend API_URL = %q", result["frontend"]["API_URL"])
	}
	if result["frontend"]["WS_URL"] != "ws://api.myproject.localtest.me:1355/ws" {
		t.Errorf("frontend WS_URL = %q", result["frontend"]["WS_URL"])
	}
	if result["worker"]["DB_PORT"] != "40123" {
		t.Errorf("worker DB_PORT = %q", result["worker"]["DB_PORT"])
	}
}

func TestResolveAllEnvLabels_Error(t *testing.T) {
	labels := map[string]map[string]string{
		"frontend": {
			"API_URL": "{{nonexistent.url}}",
		},
	}

	endpoints := map[string]ServiceEndpoint{}

	_, err := ResolveAllEnvLabels(labels, endpoints, 1355)
	if err == nil {
		t.Error("エラーを期待したがnil")
	}
}

func TestParseEnvLabels(t *testing.T) {
	dir := t.TempDir()
	composePath := filepath.Join(dir, "docker-compose.yml")

	content := `services:
  frontend:
    build: ./frontend
    ports:
      - "3000:3000"
    labels:
      portless-docker.env.API_URL: "http://{{api.host}}:{{api.port}}/v1"
      portless-docker.env.CDN_URL: "{{cdn.url}}"
      com.example.unrelated: "ignored"
  api:
    build: ./api
    ports:
      - "8080:8080"
  cdn:
    image: nginx
    ports:
      - "8081:80"
  worker:
    build: ./worker
    labels:
      portless-docker.env.API_URL: "{{api.url}}"
`
	if err := os.WriteFile(composePath, []byte(content), 0644); err != nil {
		t.Fatalf("ファイル書き込み失敗: %v", err)
	}

	result, err := ParseEnvLabels(composePath)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("2サービスを期待したが %d を取得", len(result))
	}

	fe, ok := result["frontend"]
	if !ok {
		t.Fatal("frontend のラベルを期待")
	}
	if len(fe) != 2 {
		t.Errorf("frontend: 2ラベルを期待したが %d を取得", len(fe))
	}
	if fe["API_URL"] != "http://{{api.host}}:{{api.port}}/v1" {
		t.Errorf("frontend API_URL = %q", fe["API_URL"])
	}
	if fe["CDN_URL"] != "{{cdn.url}}" {
		t.Errorf("frontend CDN_URL = %q", fe["CDN_URL"])
	}

	wk, ok := result["worker"]
	if !ok {
		t.Fatal("worker のラベルを期待")
	}
	if wk["API_URL"] != "{{api.url}}" {
		t.Errorf("worker API_URL = %q", wk["API_URL"])
	}
}

func TestGenerateOverrideWithEnvironment(t *testing.T) {
	entries := []OverrideEntry{
		{
			ServiceName:   "frontend",
			HostPort:      40001,
			ContainerPort: 3000,
			Environment: map[string]string{
				"API_URL": "http://api.myproject.localtest.me:1355/v1",
				"WS_URL":  "ws://api.myproject.localtest.me:1355/ws",
			},
		},
		{
			ServiceName:   "api",
			HostPort:      40002,
			ContainerPort: 8080,
		},
	}

	path, err := GenerateOverride(entries)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	defer func() { _ = RemoveOverride(path) }()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ファイル読み込み失敗: %v", err)
	}

	content := string(data)

	// ポートオーバーライドが含まれていること。
	if !strings.Contains(content, `"40001:3000"`) {
		t.Error("frontend のポートオーバーライドが見つかりません")
	}
	if !strings.Contains(content, `"40002:8080"`) {
		t.Error("api のポートオーバーライドが見つかりません")
	}

	// 環境変数が含まれていること。
	if !strings.Contains(content, "API_URL:") {
		t.Error("API_URL が見つかりません")
	}
	if !strings.Contains(content, "WS_URL:") {
		t.Error("WS_URL が見つかりません")
	}
	if !strings.Contains(content, "api.myproject.localtest.me:1355/v1") {
		t.Error("解決済みAPI_URL値が見つかりません")
	}
}

func TestGenerateOverrideEnvOnly(t *testing.T) {
	entries := []OverrideEntry{
		{
			ServiceName: "worker",
			Environment: map[string]string{
				"API_URL": "http://api.myproject.localtest.me:1355",
			},
		},
	}

	path, err := GenerateOverride(entries)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	defer func() { _ = RemoveOverride(path) }()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ファイル読み込み失敗: %v", err)
	}

	content := string(data)

	// ポートセクションが含まれていないこと。
	if strings.Contains(content, "ports:") {
		t.Error("ポートなしサービスに ports セクションが含まれています")
	}

	// 環境変数が含まれていること。
	if !strings.Contains(content, "API_URL:") {
		t.Error("API_URL が見つかりません")
	}
}

func TestParseEnvLabels_NoLabels(t *testing.T) {
	dir := t.TempDir()
	composePath := filepath.Join(dir, "docker-compose.yml")

	content := `services:
  api:
    ports:
      - "8080:8080"
`
	if err := os.WriteFile(composePath, []byte(content), 0644); err != nil {
		t.Fatalf("ファイル書き込み失敗: %v", err)
	}

	result, err := ParseEnvLabels(composePath)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}

	if len(result) != 0 {
		t.Errorf("空の結果を期待したが %d サービスを取得", len(result))
	}
}
