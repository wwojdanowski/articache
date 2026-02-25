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
	addrPtr := flag.String("addr", ":8080", "Artifact HTTP listen address.")
	maintenanceAddrPtr := flag.String("maintenance-addr", ":8081", "Maintenance HTTP listen address (healthz/metrics).")
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
		"maintenance_addr", *maintenanceAddrPtr,
		"cache_path", *pathPtr,
		"repo", *repoPtr,
		"workers", *workersPtr,
	)

	cache := provider.NewCache(*pathPtr, *repoPtr)
	cache.Start(*workersPtr)

	metrics.Register(prometheus.DefaultRegisterer)

	artifactMux := http.NewServeMux()
	artifactMux.HandleFunc("/", cache.HandleArtifactRequest)

	maintenanceMux := http.NewServeMux()
	maintenanceMux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	maintenanceMux.Handle("/metrics", promhttp.Handler())

	artifactServer := &http.Server{
		Addr:              *addrPtr,
		Handler:           artifactMux,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	maintenanceServer := &http.Server{
		Addr:              *maintenanceAddrPtr,
		Handler:           maintenanceMux,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = artifactServer.Shutdown(shutdownCtx)
		_ = maintenanceServer.Shutdown(shutdownCtx)
	}()

	errCh := make(chan error, 2)

	go func() {
		slog.Info("artifact server listening", "addr", artifactServer.Addr)
		if err := artifactServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	go func() {
		slog.Info("maintenance server listening", "addr", maintenanceServer.Addr)
		if err := maintenanceServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	for i := 0; i < 2; i++ {
		if err := <-errCh; err != nil {
			slog.Error("http server failed", "error", err)
			stop()
			os.Exit(1)
		}
	}
}
