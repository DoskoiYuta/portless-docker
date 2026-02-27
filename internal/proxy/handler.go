package proxy

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/DoskoiYuta/portless-docker/internal/state"
)

// Handler はリバースプロキシのHTTPハンドラーを実装する。
type Handler struct {
	stateManager *state.Manager
}

// NewHandler は新しいプロキシハンドラーを作成する。
func NewHandler(sm *state.Manager) *Handler {
	return &Handler{stateManager: sm}
}

// ServeHTTP はHostヘッダーに基づいてルーティングし、受信HTTPリクエストを処理する。
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	host := r.Host
	if host == "" {
		http.Error(w, "Hostヘッダーがありません", http.StatusBadRequest)
		return
	}

	// ホストからポート部分を除去する。
	hostname := host
	if idx := strings.LastIndex(host, ":"); idx != -1 {
		hostname = host[:idx]
	}

	routes, err := h.stateManager.GetAllRoutes()
	if err != nil {
		http.Error(w, "内部サーバーエラー", http.StatusInternalServerError)
		return
	}

	for _, route := range routes {
		if route.Hostname == hostname {
			h.proxyTo(w, r, route)
			return
		}
	}

	// 一致するルートが見つからない場合、アクティブルート一覧付きの404を返す。
	h.notFound(w, r, hostname, routes)
}

// proxyTo はリクエストをターゲットサービスに転送する。
func (h *Handler) proxyTo(w http.ResponseWriter, r *http.Request, route state.Route) {
	targetURL := &url.URL{
		Scheme: "http",
		Host:   fmt.Sprintf("127.0.0.1:%d", route.HostPort),
	}

	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	// ディレクターをカスタマイズして転送ヘッダーを設定する。
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Header.Set("X-Forwarded-For", r.RemoteAddr)
		req.Header.Set("X-Forwarded-Proto", "http")
		req.Header.Set("X-Forwarded-Host", r.Host)
		req.Header.Set("X-Portless-Docker", "1")
	}

	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusBadGateway)
		fmt.Fprintf(w, `<!DOCTYPE html>
<html><head><title>502 Bad Gateway</title>
<style>
body { font-family: -apple-system, BlinkMacSystemFont, sans-serif; max-width: 600px; margin: 50px auto; padding: 20px; }
h1 { color: #e74c3c; }
code { background: #f4f4f4; padding: 2px 6px; border-radius: 3px; }
</style></head>
<body>
<h1>502 Bad Gateway</h1>
<p>サービス <strong>%s</strong> (ポート %d) が応答していません。</p>
<p>コンテナがまだ起動中の可能性があります。数秒後にリロードしてください。</p>
</body></html>`, route.Service, route.HostPort)
	}

	proxy.ServeHTTP(w, r)
}

// notFound はアクティブルート一覧付きの404ページを返す。
func (h *Handler) notFound(w http.ResponseWriter, r *http.Request, hostname string, routes []state.Route) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)

	routeList := ""
	if len(routes) > 0 {
		routeList = "<h2>アクティブルート</h2><ul>"
		for _, route := range routes {
			routeList += fmt.Sprintf(
				`<li><a href="http://%s:%d">%s</a> → :%d (コンテナ :%d) [%s]</li>`,
				route.Hostname, 1355, route.Hostname, route.HostPort, route.ContainerPort, route.Service,
			)
		}
		routeList += "</ul>"
	} else {
		routeList = "<p>アクティブルートはありません。</p>"
	}

	fmt.Fprintf(w, `<!DOCTYPE html>
<html><head><title>404 Not Found</title>
<style>
body { font-family: -apple-system, BlinkMacSystemFont, sans-serif; max-width: 600px; margin: 50px auto; padding: 20px; }
h1 { color: #e67e22; }
a { color: #3498db; }
code { background: #f4f4f4; padding: 2px 6px; border-radius: 3px; }
</style></head>
<body>
<h1>404 Not Found</h1>
<p><code>%s</code> に対応するルートが登録されていません</p>
%s
</body></html>`, hostname, routeList)
}
