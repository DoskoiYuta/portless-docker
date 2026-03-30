package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var (
	// Version はビルド時に設定される。
	Version = "dev"
	// Commit はビルド時に設定される。
	Commit = "none"

	// グローバルフラグ
	proxyPort      int
	composeFile    string
	ignoreServices string
)

// NewRootCmd はルートのCobraコマンドを作成する。
func NewRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "portless-docker",
		Short: "Docker Compose の .localtest.me サブドメインルーター",
		Long: `portless-docker は docker-compose.yml からポートマッピングを自動検出し、
動的ホストポートを割り当て、.localtest.me サブドメイン経由でトラフィックをルーティングします。

設定ファイル不要。"docker compose" の代わりに "portless-docker" を使うだけです。`,
		Version:            fmt.Sprintf("%s (commit: %s)", Version, Commit),
		SilenceUsage:       true,
		SilenceErrors:      true,
		DisableFlagParsing: false,
		// 未知のサブコマンドに対してエラーを表示しない — パススルーで処理される。
		FParseErrWhitelist: cobra.FParseErrWhitelist{
			UnknownFlags: true,
		},
	}

	// グローバルフラグ
	rootCmd.PersistentFlags().IntVarP(&proxyPort, "port", "p", 1355, "プロキシのリッスンポート")
	rootCmd.PersistentFlags().StringVarP(&composeFile, "file", "f", "", "Composeファイルのパス")
	rootCmd.PersistentFlags().StringVar(&ignoreServices, "ignore", "", "無視するサービス（カンマ区切り）")

	// 既知のサブコマンドを登録する。
	rootCmd.AddCommand(newLsCmd())
	rootCmd.AddCommand(newStopCmd())
	rootCmd.AddCommand(newProxyCmd())

	// 未知のサブコマンドのパススルーを処理するため RunE を設定する。
	rootCmd.RunE = func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return cmd.Help()
		}
		// パススルーが以下で引数をキャッチするため、ここには到達しないはず。
		return fmt.Errorf("不明なコマンド: %s", args[0])
	}

	// 未知のサブコマンドに対するデフォルト動作をオーバーライドする。
	// Cobraはサブコマンドに一致しない場合にこれを呼び出す。
	rootCmd.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		// 未知のフラグをパススルーさせる。
		return nil
	})

	return rootCmd
}

// Execute はCLIアプリケーションを実行する。
func Execute() {
	rootCmd := NewRootCmd()

	// 最初の非フラグ引数が既知のサブコマンドかどうかを確認する。
	// そうでない場合、すべてを docker compose へのパススルーとして扱う。
	args := os.Args[1:]
	if shouldPassthrough(rootCmd, args) {
		// まずグローバルフラグを抽出し、残りをパススルーする。
		globalFlags, passthroughArgs := splitGlobalFlags(args)
		os.Args = append([]string{os.Args[0], "__passthrough"}, append(globalFlags, "--")...)
		os.Args = append(os.Args, passthroughArgs...)

		// パススルーコマンドを追加する。
		rootCmd.AddCommand(newPassthroughCmd())
	} else {
		// 既知のコマンドの場合もパススルーを追加する（必要に応じて）。
		rootCmd.AddCommand(newPassthroughCmd())
	}

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "エラー: %s\n", err)
		os.Exit(1)
	}
}

// shouldPassthrough は引数を docker compose にパススルーすべきかどうかを判定する。
func shouldPassthrough(_ *cobra.Command, args []string) bool {
	if len(args) == 0 {
		return false
	}

	// 最初の非フラグ引数を見つける。
	subcmd := findSubcommand(args)
	if subcmd == "" {
		return false
	}

	// ネイティブに処理される既知のコマンド。
	knownCmds := map[string]bool{
		"ls":            true,
		"stop":          true,
		"__passthrough": true,
		"__proxy":       true,
		"help":          true,
		"completion":    true,
	}

	return !knownCmds[subcmd]
}

// findSubcommand は最初の非フラグ引数を見つける。
func findSubcommand(args []string) string {
	skipNext := false
	for _, arg := range args {
		if skipNext {
			skipNext = false
			continue
		}
		if arg == "--" {
			return ""
		}
		if strings.HasPrefix(arg, "-") {
			// このフラグが値を取るかどうかを確認する。
			if arg == "-p" || arg == "--port" || arg == "-f" || arg == "--file" || arg == "--ignore" {
				skipNext = true
			}
			continue
		}
		return arg
	}
	return ""
}

// splitGlobalFlags はグローバルフラグとパススルー引数を分離する。
func splitGlobalFlags(args []string) (globalFlags, passthroughArgs []string) {
	skipNext := false
	foundSubcmd := false

	for i, arg := range args {
		if skipNext {
			skipNext = false
			if !foundSubcmd {
				globalFlags = append(globalFlags, arg)
			} else {
				passthroughArgs = append(passthroughArgs, arg)
			}
			continue
		}

		if foundSubcmd {
			passthroughArgs = append(passthroughArgs, arg)
			continue
		}

		if strings.HasPrefix(arg, "-") {
			if arg == "-p" || arg == "--port" || arg == "-f" || arg == "--file" || arg == "--ignore" {
				skipNext = true
				globalFlags = append(globalFlags, arg)
			} else if strings.HasPrefix(arg, "--port=") || strings.HasPrefix(arg, "--file=") ||
				strings.HasPrefix(arg, "--ignore=") || strings.HasPrefix(arg, "-p=") ||
				strings.HasPrefix(arg, "-f=") {
				globalFlags = append(globalFlags, arg)
			}
			continue
		}

		// 最初の非フラグ引数がサブコマンド。
		passthroughArgs = append(passthroughArgs, args[i:]...)
		break
	}

	return globalFlags, passthroughArgs
}

// getIgnoredServices は --ignore フラグをセットにパースする。
func getIgnoredServices() map[string]bool {
	if ignoreServices == "" {
		return nil
	}
	result := make(map[string]bool)
	for _, s := range strings.Split(ignoreServices, ",") {
		s = strings.TrimSpace(s)
		if s != "" {
			result[s] = true
		}
	}
	return result
}
