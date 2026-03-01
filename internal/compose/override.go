package compose

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// OverrideEntry はサービスのポートオーバーライドを記述する。
type OverrideEntry struct {
	ServiceName   string
	HostPort      int
	ContainerPort int
}

// GenerateOverride はホストポートを再マッピングする一時オーバーライドファイルを生成する。
// 作成されたオーバーライドファイルのパスを返す。
func GenerateOverride(entries []OverrideEntry) (string, error) {
	tmpDir, err := os.MkdirTemp("", "portless-docker-")
	if err != nil {
		return "", fmt.Errorf("一時ディレクトリの作成に失敗: %w", err)
	}

	overridePath := filepath.Join(tmpDir, "override.yml")

	// 決定的な出力のためサービス名でソートする。
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].ServiceName < entries[j].ServiceName
	})

	content := "# portless-docker により自動生成。編集しないでください。\nservices:\n"
	for _, e := range entries {
		content += fmt.Sprintf("  %s:\n    ports: !override\n      - \"%d:%d\"\n",
			e.ServiceName, e.HostPort, e.ContainerPort)
	}

	if err := os.WriteFile(overridePath, []byte(content), 0644); err != nil {
		_ = os.RemoveAll(tmpDir)
		return "", fmt.Errorf("オーバーライドファイルの書き込みに失敗: %w", err)
	}

	return overridePath, nil
}

// RemoveOverride はオーバーライドファイルとその親一時ディレクトリを削除する。
func RemoveOverride(overridePath string) error {
	if overridePath == "" {
		return nil
	}
	dir := filepath.Dir(overridePath)
	return os.RemoveAll(dir)
}
