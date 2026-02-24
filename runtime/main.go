package main

import (
	"articache/internal/logging"
	"articache/internal/metrics"
	"articache/internal/provider"
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	addrPtr := flag.String("addr", ":8080", "HTTP listen address.")
	pathPtr := flag.String("path", "/tmp/articache_data", "Cache path.")
	repoPtr := flag.String("repo", "https://repo.maven.apache.org/maven2", "Main remote repository.")
	workersPtr := flag.Int("workers", 20, "Number of background download workers.")
	logLevelPtr := flag.String("log-level", "info", "Log level: debug, info, warn, error.")
	logFormatPtr := flag.String("log-format", "json", "Log format: json or text.")
	flag.Parse()

	if err := logging.Init(*logLevelPtr, *logFormatPtr); err != nil {
		slog.Error("invalid logging configuration", "error", err)
		os.Exit(2)
	}

	slog.Info("starting articache",
		"addr", *addrPtr,
		"cache_path", *pathPtr,
		"repo", *repoPtr,
		"workers", *workersPtr,
	)

	cache := provider.NewCache(*pathPtr, *repoPtr)
	cache.Start(*workersPtr)

	metrics.Register(prometheus.DefaultRegisterer)

	mux := http.NewServeMux()
	mux.HandleFunc("/", cache.HandleArtifactRequest)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.Handle("/metrics", promhttp.Handler())

	server := &http.Server{
		Addr:              *addrPtr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	slog.Info("http server listening", "addr", server.Addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("http server failed", "error", err)
		os.Exit(1)
	}
}
