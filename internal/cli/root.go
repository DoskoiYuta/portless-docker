package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var (
	// Version is set at build time.
	Version = "dev"
	// Commit is set at build time.
	Commit = "none"

	// Global flags
	proxyPort      int
	composeFile    string
	ignoreServices string
)

// NewRootCmd creates the root cobra command.
func NewRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "portless-docker",
		Short: "Docker Compose .localhost subdomain router",
		Long: `portless-docker automatically detects port mappings from docker-compose.yml,
assigns dynamic host ports, and routes traffic via .localhost subdomains.

No config files needed. Just use "portless-docker" instead of "docker compose".`,
		Version:            fmt.Sprintf("%s (commit: %s)", Version, Commit),
		SilenceUsage:       true,
		SilenceErrors:      true,
		DisableFlagParsing: false,
		// Don't show errors for unknown subcommands — they get passed through.
		FParseErrWhitelist: cobra.FParseErrWhitelist{
			UnknownFlags: true,
		},
	}

	// Global flags
	rootCmd.PersistentFlags().IntVarP(&proxyPort, "port", "p", 1355, "Proxy listen port")
	rootCmd.PersistentFlags().StringVarP(&composeFile, "file", "f", "", "Path to compose file")
	rootCmd.PersistentFlags().StringVar(&ignoreServices, "ignore", "", "Services to ignore (comma-separated)")

	// Register known subcommands.
	rootCmd.AddCommand(newLsCmd())
	rootCmd.AddCommand(newStopCmd())
	rootCmd.AddCommand(newProxyCmd())

	// Set RunE on root to handle passthrough for unknown subcommands.
	rootCmd.RunE = func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return cmd.Help()
		}
		// This shouldn't happen since passthrough catches args below.
		return fmt.Errorf("unknown command: %s", args[0])
	}

	// Override the default behavior for unknown subcommands.
	// Cobra calls this when it doesn't match a subcommand.
	rootCmd.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		// Allow unknown flags to pass through.
		return nil
	})

	return rootCmd
}

// Execute runs the CLI application.
func Execute() {
	rootCmd := NewRootCmd()

	// Check if the first non-flag arg is a known subcommand.
	// If not, treat everything as a passthrough to docker compose.
	args := os.Args[1:]
	if shouldPassthrough(rootCmd, args) {
		// Extract our global flags first, then passthrough the rest.
		globalFlags, passthroughArgs := splitGlobalFlags(args)
		os.Args = append([]string{os.Args[0], "__passthrough"}, append(globalFlags, "--")...)
		os.Args = append(os.Args, passthroughArgs...)

		// Add the passthrough command.
		rootCmd.AddCommand(newPassthroughCmd())
	} else {
		// For known commands, add passthrough too in case it's needed.
		rootCmd.AddCommand(newPassthroughCmd())
	}

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}
}

// shouldPassthrough checks if the arguments should be passed through to docker compose.
func shouldPassthrough(rootCmd *cobra.Command, args []string) bool {
	if len(args) == 0 {
		return false
	}

	// Find the first non-flag argument.
	subcmd := findSubcommand(args)
	if subcmd == "" {
		return false
	}

	// Known commands handled natively.
	knownCmds := map[string]bool{
		"ls":             true,
		"stop":           true,
		"__passthrough":  true,
		"__proxy":        true,
		"help":           true,
		"completion":     true,
	}

	return !knownCmds[subcmd]
}

// findSubcommand finds the first non-flag argument.
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
			// Check if this flag takes a value.
			if arg == "-p" || arg == "--port" || arg == "-f" || arg == "--file" || arg == "--ignore" {
				skipNext = true
			}
			continue
		}
		return arg
	}
	return ""
}

// splitGlobalFlags separates our global flags from passthrough arguments.
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

		// First non-flag arg is the subcommand.
		foundSubcmd = true
		passthroughArgs = append(passthroughArgs, args[i:]...)
		break
	}

	return globalFlags, passthroughArgs
}

// getIgnoredServices parses the --ignore flag into a set.
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
