package server

import (
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/eduard256/claudecode2openaiapi/api"
)

const ListenAddr = ":8080"

func Init() {
	api.Use(corsMiddleware)
	api.Use(logMiddleware)

	srv := &http.Server{
		Addr:              addrFromEnv(),
		Handler:           api.Wire(),
		ReadHeaderTimeout: 30 * time.Second,
	}

	go func() {
		slog.Info("listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server stopped", "err", err)
			os.Exit(1)
		}
	}()
}

func addrFromEnv() string {
	if a := os.Getenv("CC2OA_LISTEN"); a != "" {
		return a
	}
	return ListenAddr
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		if r.Method == "OPTIONS" {
			w.WriteHeader(204)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		slog.Info("req",
			"method", r.Method,
			"path", r.URL.Path,
			"dur", time.Since(start).Round(time.Millisecond),
		)
	})
}
