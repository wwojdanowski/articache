package main

import (
	"articache/internal/provider"
	"log"
	"net/http"
)

func main() {
	log.Println("Starting Articache")
	// TODO: Make cache path configurable
	cache := provider.NewCache("/tmp", "")
	cache.Start(20)
	http.HandleFunc("/", cache.HandleArtifactRequest)
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	http.ListenAndServe(":8080", nil)
}
