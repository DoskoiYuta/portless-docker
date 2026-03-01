package ui

import (
	"fmt"
	"os"
	"os/exec"
)

// CheckDockerCompose は docker compose が利用可能かどうかを確認する。
func CheckDockerCompose() error {
	_, err := exec.LookPath("docker")
	if err != nil {
		return fmt.Errorf("docker compose がインストールされていないか PATH に含まれていません。")
	}

	// 'docker compose' サブコマンドが動作するか確認する。
	cmd := exec.Command("docker", "compose", "version")
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker compose がインストールされていないか PATH に含まれていません。")
	}

	return nil
}

// Fatal はエラーメッセージを表示して終了コード1で終了する。
func Fatal(msg string) {
	PrintError(msg)
	os.Exit(1)
}

// FatalErr はエラー値からエラーメッセージを表示して終了コード1で終了する。
func FatalErr(err error) {
	PrintError(err.Error())
	os.Exit(1)
}
