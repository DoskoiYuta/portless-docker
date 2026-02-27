package cli

import (
	"github.com/spf13/cobra"

	"github.com/DoskoiYuta/portless-docker/internal/proxy"
	"github.com/DoskoiYuta/portless-docker/internal/state"
)

// newProxyCmd creates the hidden __proxy command used to start the proxy daemon.
func newProxyCmd() *cobra.Command {
	var port int

	cmd := &cobra.Command{
		Use:    "__proxy",
		Hidden: true,
		Short:  "Start the reverse proxy server (internal use)",
		RunE: func(cmd *cobra.Command, args []string) error {
			sm, err := state.NewManager()
			if err != nil {
				return err
			}

			server := proxy.NewServer(port, sm)
			return server.Run()
		},
	}

	cmd.Flags().IntVar(&port, "port", 1355, "Proxy listen port")
	return cmd
}
