package cli

import (
	"github.com/spf13/cobra"

	"github.com/DoskoiYuta/portless-docker/internal/state"
	"github.com/DoskoiYuta/portless-docker/internal/ui"
)

func newLsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ls",
		Short: "List all active routes across all projects",
		RunE: func(cmd *cobra.Command, args []string) error {
			sm, err := state.NewManager()
			if err != nil {
				return err
			}

			routes, err := sm.GetAllRoutes()
			if err != nil {
				return err
			}

			s, err := sm.Load()
			if err != nil {
				return err
			}

			port := s.ProxyPort
			if port == 0 {
				port = proxyPort
			}

			ui.PrintActiveRoutes(routes, port)
			return nil
		},
	}
}
