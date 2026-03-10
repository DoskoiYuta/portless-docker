package compose

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const labelPrefix = "portless-docker.env."

// templatePattern は {{service.property}} プレースホルダーにマッチする。
var templatePattern = regexp.MustCompile(`\{\{(\w[\w-]*)\.(\w+)\}\}`)

// ServiceEndpoint はテンプレート解決に使用するサービスのエンドポイント情報を保持する。
type ServiceEndpoint struct {
	Hostname string
	Port     int
	Type     ServiceType
}

// ParseEnvLabels はComposeファイルの全サービスから portless-docker.env.* ラベルを読み取る。
// 戻り値: map[サービス名]map[環境変数名]テンプレート文字列
func ParseEnvLabels(composePath string) (map[string]map[string]string, error) {
	data, err := os.ReadFile(composePath)
	if err != nil {
		return nil, fmt.Errorf("composeファイルの読み込みに失敗: %w", err)
	}

	var raw struct {
		Services map[string]struct {
			Labels interface{} `yaml:"labels"`
		} `yaml:"services"`
	}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("composeファイルのパースに失敗: %w", err)
	}

	result := make(map[string]map[string]string)
	for name, svc := range raw.Services {
		labels := extractEnvLabels(svc.Labels)
		if len(labels) > 0 {
			result[name] = labels
		}
	}

	return result, nil
}

// extractEnvLabels は labels から portless-docker.env.* エントリを抽出する。
// labels は map[string]interface{} または []interface{}（"key=value" 形式）。
func extractEnvLabels(labels interface{}) map[string]string {
	result := make(map[string]string)

	switch v := labels.(type) {
	case map[string]interface{}:
		for key, val := range v {
			if strings.HasPrefix(key, labelPrefix) {
				envVar := strings.TrimPrefix(key, labelPrefix)
				if envVar != "" {
					result[envVar] = fmt.Sprintf("%v", val)
				}
			}
		}
	case []interface{}:
		for _, item := range v {
			s := fmt.Sprintf("%v", item)
			if idx := strings.Index(s, "="); idx != -1 {
				key := s[:idx]
				val := s[idx+1:]
				if strings.HasPrefix(key, labelPrefix) {
					envVar := strings.TrimPrefix(key, labelPrefix)
					if envVar != "" {
						result[envVar] = val
					}
				}
			}
		}
	}

	return result
}

// ResolveEnvTemplate は {{service.property}} プレースホルダーを解決する。
// 対応プロパティ: url, host, port
// 特殊テンプレート: {{proxy.port}} はプロキシのリッスンポートに解決される。
func ResolveEnvTemplate(tmpl string, endpoints map[string]ServiceEndpoint, proxyPort int) (string, error) {
	var resolveErr error

	result := templatePattern.ReplaceAllStringFunc(tmpl, func(match string) string {
		if resolveErr != nil {
			return match
		}

		parts := templatePattern.FindStringSubmatch(match)
		service, prop := parts[1], parts[2]

		// proxy は予約名としてプロキシポートを提供する。
		if service == "proxy" {
			if prop == "port" {
				return strconv.Itoa(proxyPort)
			}
			resolveErr = fmt.Errorf("proxy の未対応プロパティ %q（テンプレート: %s、対応: port）", prop, tmpl)
			return match
		}

		ep, ok := endpoints[service]
		if !ok {
			resolveErr = fmt.Errorf("サービス %q が見つかりません（テンプレート: %s）", service, tmpl)
			return match
		}

		switch prop {
		case "url":
			if ep.Type == ServiceTypeHTTP {
				return fmt.Sprintf("http://%s:%d", ep.Hostname, ep.Port)
			}
			return fmt.Sprintf("%s:%d", ep.Hostname, ep.Port)
		case "host":
			return ep.Hostname
		case "port":
			return strconv.Itoa(ep.Port)
		default:
			resolveErr = fmt.Errorf("未対応のプロパティ %q（テンプレート: %s）", prop, tmpl)
			return match
		}
	})

	return result, resolveErr
}

// ResolveAllEnvLabels は全サービスの環境変数テンプレートを解決する。
// 戻り値: map[サービス名]map[環境変数名]解決済み値
func ResolveAllEnvLabels(labels map[string]map[string]string, endpoints map[string]ServiceEndpoint, proxyPort int) (map[string]map[string]string, error) {
	result := make(map[string]map[string]string)

	for svcName, templates := range labels {
		resolved := make(map[string]string)
		for envVar, tmpl := range templates {
			val, err := ResolveEnvTemplate(tmpl, endpoints, proxyPort)
			if err != nil {
				return nil, fmt.Errorf("サービス %q の %s の解決に失敗: %w", svcName, envVar, err)
			}
			resolved[envVar] = val
		}
		if len(resolved) > 0 {
			result[svcName] = resolved
		}
	}

	return result, nil
}
