package proxy

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/rategate/rategate/internal/config"
)

type RouteTarget struct {
	Config config.RouteConfig
	Proxy  *httputil.ReverseProxy
}

type Router struct {
	routes []RouteTarget
}

func NewRouter(routes []config.RouteConfig) (*Router, error) {
	var targets []RouteTarget
	for _, rc := range routes {
		targetURL, err := url.Parse(rc.Upstream)
		if err != nil {
			return nil, err
		}
		proxy := httputil.NewSingleHostReverseProxy(targetURL)
		proxy.Transport = &http.Transport{
			ResponseHeaderTimeout: 5 * time.Second,
		}
		targets = append(targets, RouteTarget{Config: rc, Proxy: proxy})
	}
	return &Router{routes: targets}, nil
}

func (r *Router) Match(path string) *RouteTarget {
	for i := range r.routes {
		if strings.HasPrefix(path, r.routes[i].Config.PathPrefix) {
			return &r.routes[i]
		}
	}
	return nil
}
