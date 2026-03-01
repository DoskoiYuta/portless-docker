package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/DoskoiYuta/portless-docker/internal/state"
)

var (
	// カラー定義
	primaryColor   = lipgloss.Color("#7C3AED") // 紫
	successColor   = lipgloss.Color("#10B981") // 緑
	warningColor   = lipgloss.Color("#F59E0B") // 琥珀
	errorColor     = lipgloss.Color("#EF4444") // 赤
	mutedColor     = lipgloss.Color("#6B7280") // グレー
	linkColor      = lipgloss.Color("#3B82F6") // 青
	whiteColor     = lipgloss.Color("#F9FAFB")

	// スタイル定義
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

// RouteDisplay はルートの表示情報を保持する。
type RouteDisplay struct {
	URL           string
	HostPort      int
	ContainerPort int
	Service       string
}

// PrintBanner は portless-docker のバナーを表示する。
func PrintBanner() {
	fmt.Println(titleStyle.Render("portless-docker"))
}

// PrintComposeFile は検出されたComposeファイルのパスを表示する。
func PrintComposeFile(path string) {
	fmt.Printf("%s %s\n", labelStyle.Render("Compose:"), path)
	fmt.Println()
}

// PrintRoutes はルートマッピングをフォーマットされたテーブルで表示する。
func PrintRoutes(routes []RouteDisplay, proxyPort int) {
	if len(routes) == 0 {
		return
	}

	// 一貫した出力のためサービス名でソートする。
	sort.Slice(routes, func(i, j int) bool {
		return routes[i].Service < routes[j].Service
	})

	// 整列のため最大URL長を計算する。
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
			containerPortStyle.Render(fmt.Sprintf("(コンテナ :%d)", r.ContainerPort)),
		)
	}
	fmt.Println()
}

// PrintCommand は実行中の docker compose コマンドを表示する。
func PrintCommand(args []string) {
	cmd := strings.Join(args, " ")
	fmt.Printf("%s %s\n\n",
		labelStyle.Render("実行中:"),
		commandStyle.Render(cmd),
	)
}

// PrintDetachedMessage はバックグラウンド起動後のメッセージを表示する。
func PrintDetachedMessage() {
	fmt.Println(successStyle.Render("コンテナをバックグラウンドで起動しました。"))
	fmt.Printf("停止するには %s を実行してください。\n", commandStyle.Render("portless-docker down"))
}

// PrintCleanup はクリーンアップの概要を表示する。
func PrintCleanup(routeCount int) {
	fmt.Println()
	fmt.Println(successStyle.Render(
		fmt.Sprintf("クリーンアップ完了: オーバーライド削除、%d 件のルートを登録解除。", routeCount),
	))
}

// PrintProxyStopped はプロキシ停止メッセージを表示する。
func PrintProxyStopped() {
	fmt.Println(successStyle.Render("残存ルートなし。プロキシを停止しました。"))
}

// PrintStopping はコンテナ停止中メッセージを表示する。
func PrintStopping() {
	fmt.Println()
	fmt.Println(warningStyle.Render("コンテナを停止中..."))
}

// PrintActiveRoutes はディレクトリ別にグループ化された全アクティブルートを表示する。
func PrintActiveRoutes(routes []state.Route, proxyPort int) {
	if len(routes) == 0 {
		fmt.Println(warningStyle.Render("アクティブルートはありません。"))
		return
	}

	fmt.Println(titleStyle.Render("アクティブルート:"))
	fmt.Println()

	// ディレクトリ別にグループ化する。
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
				containerPortStyle.Render(fmt.Sprintf("(コンテナ :%d)", r.ContainerPort)),
			)
		}
		fmt.Println()
	}
}

// PrintError はフォーマットされたエラーメッセージを表示する。
func PrintError(msg string) {
	fmt.Println(errorStyle.Render("エラー: " + msg))
}

// PrintWarning はフォーマットされた警告メッセージを表示する。
func PrintWarning(msg string) {
	fmt.Println(warningStyle.Render("警告: " + msg))
}

// PrintSuccess はフォーマットされた成功メッセージを表示する。
func PrintSuccess(msg string) {
	fmt.Println(successStyle.Render(msg))
}
