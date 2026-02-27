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
		Short: "Stop portless-docker and unregister routes",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !all {
				return fmt.Errorf("use --all to stop all routes and the proxy")
			}

			sm, err := state.NewManager()
			if err != nil {
				return err
			}

			// Remove all routes.
			removed, err := sm.UnregisterAllRoutes()
			if err != nil {
				return err
			}

			// Clean up override files.
			seen := make(map[string]bool)
			for _, r := range removed {
				if r.OverridePath != "" && !seen[r.OverridePath] {
					seen[r.OverridePath] = true
					compose.RemoveOverride(r.OverridePath)
				}
			}

			ui.PrintCleanup(len(removed))

			// Stop the proxy daemon.
			daemon := proxy.NewDaemon(sm)
			if daemon.IsRunning() {
				if err := daemon.Stop(); err != nil {
					return fmt.Errorf("failed to stop proxy: %w", err)
				}
				ui.PrintProxyStopped()
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&all, "all", false, "Stop all routes and the proxy")
	return cmd
}
