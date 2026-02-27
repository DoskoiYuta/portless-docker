package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/DoskoiYuta/portless-docker/internal/state"
)

var (
	// Colors
	primaryColor   = lipgloss.Color("#7C3AED") // Purple
	successColor   = lipgloss.Color("#10B981") // Green
	warningColor   = lipgloss.Color("#F59E0B") // Amber
	errorColor     = lipgloss.Color("#EF4444") // Red
	mutedColor     = lipgloss.Color("#6B7280") // Gray
	linkColor      = lipgloss.Color("#3B82F6") // Blue
	whiteColor     = lipgloss.Color("#F9FAFB")

	// Styles
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor).
			MarginBottom(1)

	labelStyle = lipgloss.NewStyle().
			Foreground(mutedColor)

	urlStyle = lipgloss.NewStyle().
			Foreground(linkColor).
			Bold(true)

	arrowStyle = lipgloss.NewStyle().
			Foreground(mutedColor)

	portStyle = lipgloss.NewStyle().
			Foreground(successColor)

	containerPortStyle = lipgloss.NewStyle().
				Foreground(mutedColor)

	commandStyle = lipgloss.NewStyle().
			Foreground(whiteColor).
			Italic(true)

	successStyle = lipgloss.NewStyle().
			Foreground(successColor)

	errorStyle = lipgloss.NewStyle().
			Foreground(errorColor).
			Bold(true)

	warningStyle = lipgloss.NewStyle().
			Foreground(warningColor)

	dirStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true)

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(primaryColor).
			Padding(0, 1)
)

// RouteDisplay holds the display info for a route.
type RouteDisplay struct {
	URL           string
	HostPort      int
	ContainerPort int
	Service       string
}

// PrintBanner prints the portless-docker banner.
func PrintBanner() {
	fmt.Println(titleStyle.Render("portless-docker"))
}

// PrintComposeFile prints the detected compose file path.
func PrintComposeFile(path string) {
	fmt.Printf("%s %s\n", labelStyle.Render("Compose:"), path)
	fmt.Println()
}

// PrintRoutes prints the route mappings in a formatted table.
func PrintRoutes(routes []RouteDisplay, proxyPort int) {
	if len(routes) == 0 {
		return
	}

	// Sort routes by service name for consistent output.
	sort.Slice(routes, func(i, j int) bool {
		return routes[i].Service < routes[j].Service
	})

	// Calculate max URL length for alignment.
	maxURLLen := 0
	for _, r := range routes {
		url := fmt.Sprintf("http://%s:%d", r.URL, proxyPort)
		if len(url) > maxURLLen {
			maxURLLen = len(url)
		}
	}

	for _, r := range routes {
		url := fmt.Sprintf("http://%s:%d", r.URL, proxyPort)
		padding := strings.Repeat(" ", maxURLLen-len(url))
		fmt.Printf("  %s%s  %s  %s %s\n",
			urlStyle.Render(url),
			padding,
			arrowStyle.Render("→"),
			portStyle.Render(fmt.Sprintf(":%d", r.HostPort)),
			containerPortStyle.Render(fmt.Sprintf("(container :%d)", r.ContainerPort)),
		)
	}
	fmt.Println()
}

// PrintCommand prints the docker compose command being executed.
func PrintCommand(args []string) {
	cmd := strings.Join(args, " ")
	fmt.Printf("%s %s\n\n",
		labelStyle.Render("Running:"),
		commandStyle.Render(cmd),
	)
}

// PrintDetachedMessage prints the message shown after starting in detached mode.
func PrintDetachedMessage() {
	fmt.Println(successStyle.Render("Containers started in background."))
	fmt.Printf("Run %s to stop.\n", commandStyle.Render("portless-docker down"))
}

// PrintCleanup prints the cleanup summary.
func PrintCleanup(routeCount int) {
	fmt.Println()
	fmt.Println(successStyle.Render(
		fmt.Sprintf("Cleaned up: override removed, %d route(s) unregistered.", routeCount),
	))
}

// PrintProxyStopped prints the proxy stopped message.
func PrintProxyStopped() {
	fmt.Println(successStyle.Render("No routes remaining. Proxy stopped."))
}

// PrintStopping prints the stopping message.
func PrintStopping() {
	fmt.Println()
	fmt.Println(warningStyle.Render("Stopping containers..."))
}

// PrintActiveRoutes prints all active routes grouped by directory.
func PrintActiveRoutes(routes []state.Route, proxyPort int) {
	if len(routes) == 0 {
		fmt.Println(warningStyle.Render("No active routes."))
		return
	}

	fmt.Println(titleStyle.Render("Active routes:"))
	fmt.Println()

	// Group by directory.
	grouped := make(map[string][]state.Route)
	var dirs []string
	for _, r := range routes {
		if _, ok := grouped[r.Directory]; !ok {
			dirs = append(dirs, r.Directory)
		}
		grouped[r.Directory] = append(grouped[r.Directory], r)
	}
	sort.Strings(dirs)

	for _, dir := range dirs {
		fmt.Printf(" %s\n", dirStyle.Render(dir))

		dirRoutes := grouped[dir]
		sort.Slice(dirRoutes, func(i, j int) bool {
			return dirRoutes[i].Service < dirRoutes[j].Service
		})

		maxURLLen := 0
		for _, r := range dirRoutes {
			url := fmt.Sprintf("http://%s:%d", r.Hostname, proxyPort)
			if len(url) > maxURLLen {
				maxURLLen = len(url)
			}
		}

		for _, r := range dirRoutes {
			url := fmt.Sprintf("http://%s:%d", r.Hostname, proxyPort)
			padding := strings.Repeat(" ", maxURLLen-len(url))
			fmt.Printf("   %s%s  %s  %s %s\n",
				urlStyle.Render(url),
				padding,
				arrowStyle.Render("→"),
				portStyle.Render(fmt.Sprintf(":%d", r.HostPort)),
				containerPortStyle.Render(fmt.Sprintf("(container :%d)", r.ContainerPort)),
			)
		}
		fmt.Println()
	}
}

// PrintError prints a formatted error message.
func PrintError(msg string) {
	fmt.Println(errorStyle.Render("Error: " + msg))
}

// PrintWarning prints a formatted warning message.
func PrintWarning(msg string) {
	fmt.Println(warningStyle.Render("Warning: " + msg))
}

// PrintSuccess prints a formatted success message.
func PrintSuccess(msg string) {
	fmt.Println(successStyle.Render(msg))
}
