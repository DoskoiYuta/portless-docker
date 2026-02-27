package cli

import (
	"github.com/spf13/cobra"

	"github.com/DoskoiYuta/portless-docker/internal/proxy"
	"github.com/DoskoiYuta/portless-docker/internal/state"
)

// newProxyCmd はプロキシデーモン起動用の隠し __proxy コマンドを作成する。
func newProxyCmd() *cobra.Command {
	var port int

	cmd := &cobra.Command{
		Use:    "__proxy",
		Hidden: true,
		Short:  "リバースプロキシサーバーを起動する（内部使用）",
		RunE: func(cmd *cobra.Command, args []string) error {
			sm, err := state.NewManager()
			if err != nil {
				return err
			}

			server := proxy.NewServer(port, sm)
			return server.Run()
		},
	}

	cmd.Flags().IntVar(&port, "port", 1355, "プロキシのリッスンポート")
	return cmd
}
