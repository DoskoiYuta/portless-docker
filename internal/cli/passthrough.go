package cli

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
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
		DisableFlagParsing: false,
		Args:               cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Separate passthrough args (after "--").
			var passthroughArgs []string
			for i, a := range args {
				if a == "--" {
					passthroughArgs = args[i+1:]
					break
				}
			}

			if len(passthroughArgs) == 0 {
				return fmt.Errorf("no docker compose subcommand specified")
			}

			return runPassthrough(passthroughArgs)
		},
	}
	return cmd
}

func runPassthrough(args []string) error {
	// Check docker compose is available.
	if err := ui.CheckDockerCompose(); err != nil {
		return err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
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

	// Determine compose file.
	composePath, err := compose.FindComposeFile(cwd, composeFile)
	if err != nil {
		return err
	}

	// Check if routes are already registered for this directory.
	existingRoutes, err := sm.GetRoutes(cwd)
	if err != nil {
		return err
	}

	var overridePath string
	var routes []state.Route
	needsSetup := len(existingRoutes) == 0

	if needsSetup {
		// Parse compose file and set up routes.
		cf, err := compose.Parse(composePath, getIgnoredServices())
		if err != nil {
			return err
		}

		// Get already used ports.
		usedPorts, err := sm.GetUsedPorts()
		if err != nil {
			return err
		}

		// Allocate ports.
		allocator := ports.NewAllocator(usedPorts)
		var overrideEntries []compose.OverrideEntry

		// Sort service names for deterministic output.
		var serviceNames []string
		for name := range cf.Services {
			serviceNames = append(serviceNames, name)
		}
		sort.Strings(serviceNames)

		for _, name := range serviceNames {
			svc := cf.Services[name]
			hostPort, err := allocator.Allocate()
			if err != nil {
				return fmt.Errorf("failed to allocate port for %s: %w", name, err)
			}

			hostname := compose.ServiceSubdomain(name) + ".localhost"

			routes = append(routes, state.Route{
				Hostname:      hostname,
				HostPort:      hostPort,
				ContainerPort: svc.ContainerPort,
				Service:       name,
				Directory:     cwd,
				ComposeFile:   composePath,
			})

			overrideEntries = append(overrideEntries, compose.OverrideEntry{
				ServiceName:   name,
				HostPort:      hostPort,
				ContainerPort: svc.ContainerPort,
			})
		}

		// Generate override file.
		overridePath, err = compose.GenerateOverride(overrideEntries)
		if err != nil {
			return fmt.Errorf("failed to generate override: %w", err)
		}

		// Set override path on all routes.
		for i := range routes {
			routes[i].OverridePath = overridePath
		}

		// Determine detached mode.
		isDetached := isDetachedMode(args)
		for i := range routes {
			routes[i].Detached = isDetached
		}

		// Register routes.
		if err := sm.RegisterRoutes(routes); err != nil {
			compose.RemoveOverride(overridePath)
			return err
		}

		// Ensure proxy is running.
		daemon := proxy.NewDaemon(sm)
		if err := daemon.EnsureRunning(proxyPort); err != nil {
			return err
		}

		// Print banner and route info.
		ui.PrintBanner()
		ui.PrintComposeFile(composePath)

		var displays []ui.RouteDisplay
		for _, r := range routes {
			displays = append(displays, ui.RouteDisplay{
				URL:           r.Hostname,
				HostPort:      r.HostPort,
				ContainerPort: r.ContainerPort,
				Service:       r.Service,
			})
		}
		ui.PrintRoutes(displays, proxyPort)
	} else {
		routes = existingRoutes
		overridePath = existingRoutes[0].OverridePath
	}

	// Build docker compose command.
	composeArgs := buildComposeArgs(composePath, overridePath, args)

	if needsSetup {
		ui.PrintCommand(append([]string{"docker", "compose"}, composeArgs...))
	}

	// Execute docker compose.
	exitCode := execDockerCompose(composeArgs)

	// Post-processing.
	isUp := subcmd == "up"
	isDown := subcmd == "down"
	isForegroundUp := isUp && !isDetachedMode(args)

	if isDown || isForegroundUp {
		cleanup(sm, cwd, overridePath, len(routes))
	}

	if isUp && isDetachedMode(args) && needsSetup {
		ui.PrintDetachedMessage()
	}

	if exitCode != 0 {
		os.Exit(exitCode)
	}

	return nil
}

// buildComposeArgs builds the docker compose command arguments.
func buildComposeArgs(composePath, overridePath string, userArgs []string) []string {
	args := []string{"-f", composePath}
	if overridePath != "" {
		args = append(args, "-f", overridePath)
	}
	args = append(args, userArgs...)
	return args
}

// execDockerCompose runs docker compose and returns the exit code.
func execDockerCompose(args []string) int {
	cmd := exec.Command("docker", append([]string{"compose"}, args...)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Forward signals to the child process.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		for sig := range sigCh {
			if cmd.Process != nil {
				cmd.Process.Signal(sig)
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

// cleanup removes override files and unregisters routes.
func cleanup(sm *state.Manager, directory, overridePath string, routeCount int) {
	ui.PrintStopping()

	// Unregister routes.
	sm.UnregisterRoutes(directory)

	// Remove override file.
	compose.RemoveOverride(overridePath)

	ui.PrintCleanup(routeCount)

	// Check if proxy should be stopped.
	has, err := sm.HasRoutes()
	if err == nil && !has {
		daemon := proxy.NewDaemon(sm)
		if daemon.IsRunning() {
			daemon.Stop()
			ui.PrintProxyStopped()
		}
	}
}

// isDetachedMode checks if -d flag is present in args.
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

// isUpCommand checks if the subcommand is "up" (first non-flag arg).
func isUpCommand(args []string) bool {
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			continue
		}
		return arg == "up"
	}
	return false
}
