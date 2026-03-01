package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/DoskoiYuta/portless-docker/internal/compose"
	"github.com/DoskoiYuta/portless-docker/internal/proxy"
	"github.com/DoskoiYuta/portless-docker/internal/state"
	"github.com/DoskoiYuta/portless-docker/internal/ui"
)

func newStopCmd() *cobra.Command {
	var all bool

	cmd := &cobra.Command{
		Use:   "stop",
		Short: "portless-docker を停止してルートを登録解除する",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !all {
				return fmt.Errorf("全ルートとプロキシを停止するには --all を使用してください")
			}

			sm, err := state.NewManager()
			if err != nil {
				return err
			}

			// 全ルートを削除する。
			removed, err := sm.UnregisterAllRoutes()
			if err != nil {
				return err
			}

			// オーバーライドファイルをクリーンアップする。
			seen := make(map[string]bool)
			for _, r := range removed {
				if r.OverridePath != "" && !seen[r.OverridePath] {
					seen[r.OverridePath] = true
					_ = compose.RemoveOverride(r.OverridePath)
				}
			}

			ui.PrintCleanup(len(removed))

			// プロキシデーモンを停止する。
			daemon := proxy.NewDaemon(sm)
			if daemon.IsRunning() {
				if err := daemon.Stop(); err != nil {
					return fmt.Errorf("プロキシの停止に失敗: %w", err)
				}
				ui.PrintProxyStopped()
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&all, "all", false, "全ルートとプロキシを停止する")
	return cmd
}
