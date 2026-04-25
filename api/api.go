// Package api is the registry for HTTP routes and middleware. Modules
// register themselves here in their Init(). The server then wires everything
// onto http.DefaultServeMux and applies the middleware chain.
package api

import "net/http"

var (
	routes      = map[string]http.HandlerFunc{}
	middlewares []func(http.Handler) http.Handler
)

// HandleFunc registers a handler for an exact path. Duplicate registration
// silently overwrites the prior handler.
func HandleFunc(path string, h http.HandlerFunc) {
	routes[path] = h
}

// Use appends a middleware. They wrap in registration order: the first
// registered middleware is the outermost.
func Use(m func(http.Handler) http.Handler) {
	middlewares = append(middlewares, m)
}

// Wire returns a single http.Handler with all registered routes and the
// middleware chain applied.
func Wire() http.Handler {
	mux := http.NewServeMux()
	for path, h := range routes {
		mux.HandleFunc(path, h)
	}

	var h http.Handler = mux
	for i := len(middlewares) - 1; i >= 0; i-- {
		h = middlewares[i](h)
	}
	return h
}
