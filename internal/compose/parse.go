package compose

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// ServiceType はサービスのプロトコル種別を表す。
type ServiceType string

const (
	// ServiceTypeHTTP は HTTP プロキシ経由でルーティングされるサービス。
	ServiceTypeHTTP ServiceType = "http"
	// ServiceTypeTCP はポートマッピングのみで直接アクセスされる TCP サービス。
	ServiceTypeTCP ServiceType = "tcp"
)

// wellKnownTCPPorts は TCP として自動判定される well-known ポート。
var wellKnownTCPPorts = map[int]bool{
	3306:  true, // MySQL
	5432:  true, // PostgreSQL
	6379:  true, // Redis
	27017: true, // MongoDB
	9042:  true, // Cassandra
	5672:  true, // RabbitMQ (AMQP)
	11211: true, // Memcached
	2181:  true, // ZooKeeper
	9092:  true, // Kafka
	6380:  true, // Redis (alt)
	26379: true, // Redis Sentinel
}

// IsWellKnownTCPPort は指定ポートが well-known TCP ポートかどうかを判定する。
func IsWellKnownTCPPort(port int) bool {
	return wellKnownTCPPorts[port]
}

// ClassifyPort はコンテナポートからサービス種別を判定する。
func ClassifyPort(containerPort int) ServiceType {
	if IsWellKnownTCPPort(containerPort) {
		return ServiceTypeTCP
	}
	return ServiceTypeHTTP
}

// ComposeFile はパース済みの docker-compose.yml を表す。
type ComposeFile struct {
	Path     string
	Services map[string]ServiceDef
}

// ServiceDef はComposeファイル内のサービス定義を表す。
type ServiceDef struct {
	Name          string
	ContainerPort int
	HostPort      int
	RawPorts      []string
	Type          ServiceType
}

// PortMapping はパース済みのポートマッピングを表す。
type PortMapping struct {
	HostPort      int
	ContainerPort int
	Protocol      string
}

// composeFileNames は検索対象のComposeファイル名の優先順リスト。
var composeFileNames = []string{
	"docker-compose.yml",
	"docker-compose.yaml",
	"compose.yml",
	"compose.yaml",
}

// FindComposeFile は指定ディレクトリ内でComposeファイルを探す。
// filePath が指定されている場合はそのパスを直接使用する。
func FindComposeFile(dir, filePath string) (string, error) {
	if filePath != "" {
		abs, err := filepath.Abs(filePath)
		if err != nil {
			return "", fmt.Errorf("無効なパス: %w", err)
		}
		if _, err := os.Stat(abs); err != nil {
			return "", fmt.Errorf("composeファイルが見つかりません: %s", abs)
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

	return "", fmt.Errorf("docker-compose.yml が見つかりません。")
}

// Parse はdocker-compose.ymlファイルを読み込みパースし、ポートマッピングを持つサービスを返す。
// ignoredServices はスキップするサービス名のセット。
func Parse(composePath string, ignoredServices map[string]bool) (*ComposeFile, error) {
	data, err := os.ReadFile(composePath)
	if err != nil {
		return nil, fmt.Errorf("composeファイルの読み込みに失敗: %w", err)
	}

	var raw struct {
		Services map[string]struct {
			Ports []interface{} `yaml:"ports"`
		} `yaml:"services"`
	}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("composeファイルのパースに失敗: %w", err)
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
			Type:          ClassifyPort(pm.ContainerPort),
		}
	}

	if len(cf.Services) == 0 {
		return nil, fmt.Errorf("ポートマッピングを持つサービスが見つかりません。")
	}

	return cf, nil
}

// parseFirstTCPPort はポート文字列リストから最初のTCPポートマッピングを抽出する。
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

// parsePortString は単一のDocker Composeポート文字列をパースする。
// 対応フォーマット:
//   - "3000"
//   - "3000:3000"
//   - "8080:80"
//   - "127.0.0.1:3000:3000"
//   - "0.0.0.0:3000:3000/tcp"
func parsePortString(s string) *PortMapping {
	// プロトコルサフィックスを除去する。
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
		// "3000" — ホストポートとコンテナポートが同一
		hostPort, err = strconv.Atoi(parts[0])
		if err != nil {
			return nil
		}
		containerPort = hostPort
	case 2:
		// "8080:80" — ホスト:コンテナ
		hostPort, err = strconv.Atoi(parts[0])
		if err != nil {
			return nil
		}
		containerPort, err = strconv.Atoi(parts[1])
		if err != nil {
			return nil
		}
	case 3:
		// "127.0.0.1:3000:3000" — 先頭はIPアドレスなのでスキップ
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

// ServiceSubdomain はサービス名を有効なサブドメインに変換する。
// アンダースコアはハイフンに、大文字は小文字に変換される。
func ServiceSubdomain(name string) string {
	s := strings.ReplaceAll(name, "_", "-")
	s = strings.ToLower(s)
	return s
}

// BuildHostname はサービス名とプロジェクト名からホスト名を生成する。
// 形式: <service>.<project>.localhost
func BuildHostname(serviceName, projectName string) string {
	return ServiceSubdomain(serviceName) + "." + ServiceSubdomain(projectName) + ".localhost"
}
