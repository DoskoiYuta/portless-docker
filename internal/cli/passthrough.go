package cli

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/DoskoiYuta/portless-docker/internal/compose"
	"github.com/DoskoiYuta/portless-docker/internal/ports"
	"github.com/DoskoiYuta/portless-docker/internal/proxy"
	"github.com/DoskoiYuta/portless-docker/internal/state"
	"github.com/DoskoiYuta/portless-docker/internal/ui"
)

func newPassthroughCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                "__passthrough",
		Hidden:             true,
		DisableFlagParsing: true,
		Args:               cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			// パススルー引数を分離する（"--" の後）。
			var passthroughArgs []string
			for i, a := range args {
				if a == "--" {
					passthroughArgs = args[i+1:]
					break
				}
			}

			if len(passthroughArgs) == 0 {
				return fmt.Errorf("docker compose のサブコマンドが指定されていません")
			}

			return runPassthrough(passthroughArgs)
		},
	}
	return cmd
}

func runPassthrough(args []string) error {
	// docker compose が利用可能か確認する。
	if err := ui.CheckDockerCompose(); err != nil {
		return err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("作業ディレクトリの取得に失敗: %w", err)
	}
	cwd, _ = filepath.Abs(cwd)

	sm, err := state.NewManager()
	if err != nil {
		return err
	}

	subcmd := ""
	if len(args) > 0 {
		subcmd = args[0]
	}

	// Composeファイルを決定する。
	composePath, err := compose.FindComposeFile(cwd, composeFile)
	if err != nil {
		return err
	}

	// このディレクトリにルートが既に登録されているか確認する。
	existingRoutes, err := sm.GetRoutes(cwd)
	if err != nil {
		return err
	}

	var overridePath string
	var routes []state.Route
	needsSetup := len(existingRoutes) == 0

	if needsSetup {
		// Composeファイルをパースしてルートをセットアップする。
		cf, err := compose.Parse(composePath, getIgnoredServices())
		if err != nil {
			return err
		}

		// 既に使用中のポートを取得する。
		usedPorts, err := sm.GetUsedPorts()
		if err != nil {
			return err
		}

		// ポートを割り当てる。
		allocator := ports.NewAllocator(usedPorts)
		var overrideEntries []compose.OverrideEntry

		// 決定的な出力のためサービス名をソートする。
		var serviceNames []string
		for name := range cf.Services {
			serviceNames = append(serviceNames, name)
		}
		sort.Strings(serviceNames)

		// プロジェクト名を決定論的ポート割り当てのキーに使用する。
		projectName := filepath.Base(cwd)

		for _, name := range serviceNames {
			svc := cf.Services[name]
			serviceType := string(svc.Type)

			var hostPort int
			if svc.Type == compose.ServiceTypeTCP {
				// TCP サービス: FNV-1a ハッシュによる決定論的ポート割り当て。
				key := fmt.Sprintf("%s:%s:%d", projectName, name, svc.ContainerPort)
				hostPort, err = allocator.AllocateDeterministic(key)
			} else {
				// HTTP サービス: 従来の空きポート順次割り当て。
				hostPort, err = allocator.Allocate()
			}
			if err != nil {
				return fmt.Errorf("%s のポート割り当てに失敗: %w", name, err)
			}

			hostname := compose.BuildHostname(name, projectName)

			routes = append(routes, state.Route{
				Hostname:      hostname,
				HostPort:      hostPort,
				ContainerPort: svc.ContainerPort,
				Service:       name,
				Type:          serviceType,
				Directory:     cwd,
				ComposeFile:   composePath,
			})

			overrideEntries = append(overrideEntries, compose.OverrideEntry{
				ServiceName:   name,
				HostPort:      hostPort,
				ContainerPort: svc.ContainerPort,
			})
		}

		// ラベルから環境変数テンプレートを解決する。
		envLabels, err := compose.ParseEnvLabels(composePath)
		if err != nil {
			return err
		}
		if len(envLabels) > 0 {
			endpoints := make(map[string]compose.ServiceEndpoint)
			for _, r := range routes {
				ep := compose.ServiceEndpoint{
					Hostname: r.Hostname,
					Port:     proxyPort,
					Type:     compose.ServiceType(r.Type),
				}
				if r.Type == string(compose.ServiceTypeTCP) {
					ep.Hostname = "localhost"
					ep.Port = r.HostPort
				}
				endpoints[r.Service] = ep
			}

			resolved, err := compose.ResolveAllEnvLabels(envLabels, endpoints)
			if err != nil {
				return err
			}

			// 既存のオーバーライドエントリに環境変数を追加する。
			for i, entry := range overrideEntries {
				if env, ok := resolved[entry.ServiceName]; ok {
					overrideEntries[i].Environment = env
					delete(resolved, entry.ServiceName)
				}
			}

			// ポートなし・ラベルありのサービス用エントリを追加する。
			for svcName, env := range resolved {
				overrideEntries = append(overrideEntries, compose.OverrideEntry{
					ServiceName: svcName,
					Environment: env,
				})
			}
		}

		// オーバーライドファイルを生成する。
		overridePath, err = compose.GenerateOverride(overrideEntries)
		if err != nil {
			return fmt.Errorf("オーバーライドの生成に失敗: %w", err)
		}

		// 全ルートにオーバーライドパスを設定する。
		for i := range routes {
			routes[i].OverridePath = overridePath
		}

		// デタッチモードを判定する。
		isDetached := isDetachedMode(args)
		for i := range routes {
			routes[i].Detached = isDetached
		}

		// ルートを登録する。
		if err := sm.RegisterRoutes(routes); err != nil {
			_ = compose.RemoveOverride(overridePath)
			return err
		}

		// プロキシが起動していることを確認する。
		daemon := proxy.NewDaemon(sm)
		if err := daemon.EnsureRunning(proxyPort); err != nil {
			return err
		}

		// バナーとルート情報を表示する。
		ui.PrintBanner()
		ui.PrintComposeFile(composePath)

		var displays []ui.RouteDisplay
		for _, r := range routes {
			displays = append(displays, ui.RouteDisplay{
				URL:           r.Hostname,
				HostPort:      r.HostPort,
				ContainerPort: r.ContainerPort,
				Service:       r.Service,
				Type:          r.Type,
			})
		}
		ui.PrintRoutes(displays, proxyPort)
	} else {
		routes = existingRoutes
		overridePath = existingRoutes[0].OverridePath
	}

	// docker compose コマンドを構築する。
	composeArgs := buildComposeArgs(composePath, overridePath, args)

	if needsSetup {
		ui.PrintCommand(append([]string{"docker", "compose"}, composeArgs...))
	}

	// docker compose を実行する。
	exitCode := execDockerCompose(composeArgs)

	// 後処理。
	if shouldCleanup(subcmd, args) {
		cleanup(sm, cwd, overridePath, len(routes))
	}

	if subcmd == "up" && isDetachedMode(args) && needsSetup {
		ui.PrintDetachedMessage()
	}

	if exitCode != 0 {
		os.Exit(exitCode)
	}

	return nil
}

// buildComposeArgs は docker compose コマンドの引数を構築する。
func buildComposeArgs(composePath, overridePath string, userArgs []string) []string {
	args := []string{"-f", composePath}
	if overridePath != "" {
		args = append(args, "-f", overridePath)
	}
	args = append(args, userArgs...)
	return args
}

// execDockerCompose は docker compose を実行して終了コードを返す。
func execDockerCompose(args []string) int {
	cmd := exec.Command("docker", append([]string{"compose"}, args...)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// シグナルを子プロセスに転送する。
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		for sig := range sigCh {
			if cmd.Process != nil {
				_ = cmd.Process.Signal(sig)
			}
		}
	}()

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode()
		}
		return 1
	}
	return 0
}

// cleanup はオーバーライドファイルを削除し、ルートを登録解除する。
func cleanup(sm *state.Manager, directory, overridePath string, routeCount int) {
	ui.PrintStopping()

	// ルートを登録解除する。
	_, _ = sm.UnregisterRoutes(directory)

	// オーバーライドファイルを削除する。
	_ = compose.RemoveOverride(overridePath)

	ui.PrintCleanup(routeCount)

	// プロキシを停止すべきか確認する。
	has, err := sm.HasRoutes()
	if err == nil && !has {
		daemon := proxy.NewDaemon(sm)
		if daemon.IsRunning() {
			_ = daemon.Stop()
			ui.PrintProxyStopped()
		}
	}
}

// shouldCleanup はサブコマンドに応じてクリーンアップが必要かを判定する。
func shouldCleanup(subcmd string, args []string) bool {
	switch subcmd {
	case "down", "stop":
		return true
	case "up":
		return !isDetachedMode(args)
	default:
		return false
	}
}

// isDetachedMode は引数に -d フラグが含まれているかを確認する。
func isDetachedMode(args []string) bool {
	for _, arg := range args {
		if arg == "-d" || arg == "--detach" {
			return true
		}
		if arg == "--" {
			return false
		}
	}
	return false
}
