package main

import (
	"articache/internal/metrics"
	"articache/internal/provider"
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	log.Println("Starting Articache")
	addrPtr := flag.String("addr", ":8080", "HTTP listen address.")
	pathPtr := flag.String("path", "/tmp/articache_data", "Cache path.")
	repoPtr := flag.String("repo", "https://repo.maven.apache.org/maven2", "Main remote repository.")
	workersPtr := flag.Int("workers", 20, "Number of background download workers.")
	flag.Parse()

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

	log.Printf("Listening on %s", server.Addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("HTTP server failed: %v", err)
	}
}
