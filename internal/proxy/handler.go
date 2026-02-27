package proxy

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/DoskoiYuta/portless-docker/internal/state"
)

// Handler implements the HTTP handler for the reverse proxy.
type Handler struct {
	stateManager *state.Manager
}

// NewHandler creates a new proxy handler.
func NewHandler(sm *state.Manager) *Handler {
	return &Handler{stateManager: sm}
}

// ServeHTTP handles incoming HTTP requests by routing based on the Host header.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	host := r.Host
	if host == "" {
		http.Error(w, "Missing Host header", http.StatusBadRequest)
		return
	}

	// Strip port from host if present.
	hostname := host
	if idx := strings.LastIndex(host, ":"); idx != -1 {
		hostname = host[:idx]
	}

	routes, err := h.stateManager.GetAllRoutes()
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	for _, route := range routes {
		if route.Hostname == hostname {
			h.proxyTo(w, r, route)
			return
		}
	}

	// No matching route found — return 404 with active routes list.
	h.notFound(w, r, hostname, routes)
}

// proxyTo forwards the request to the target service.
func (h *Handler) proxyTo(w http.ResponseWriter, r *http.Request, route state.Route) {
	targetURL := &url.URL{
		Scheme: "http",
		Host:   fmt.Sprintf("127.0.0.1:%d", route.HostPort),
	}

	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	// Customize the director to set forwarding headers.
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
<p>Service <strong>%s</strong> (port %d) is not responding.</p>
<p>The container may still be starting up. Try refreshing in a few seconds.</p>
</body></html>`, route.Service, route.HostPort)
	}

	proxy.ServeHTTP(w, r)
}

// notFound returns a 404 page listing active routes.
func (h *Handler) notFound(w http.ResponseWriter, r *http.Request, hostname string, routes []state.Route) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)

	routeList := ""
	if len(routes) > 0 {
		routeList = "<h2>Active Routes</h2><ul>"
		for _, route := range routes {
			routeList += fmt.Sprintf(
				`<li><a href="http://%s:%d">%s</a> → :%d (container :%d) [%s]</li>`,
				route.Hostname, 1355, route.Hostname, route.HostPort, route.ContainerPort, route.Service,
			)
		}
		routeList += "</ul>"
	} else {
		routeList = "<p>No active routes.</p>"
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
<p>No route registered for <code>%s</code></p>
%s
</body></html>`, hostname, routeList)
}
