package main

import (
	// provider "articache/internal/provider"
	"log"
	"net/http"
)

func main() {
	log.Println("Starting Articache")
	// TODO: Make cache path configurable
	cache := Cache{"/tmp", make(chan artifactPath), http.DefaultClient}
	cache.downloadLoop(20, cache.queue)
	http.HandleFunc("/", cache.HandleArtifactRequest)
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	http.ListenAndServe(":8080", nil)
}
