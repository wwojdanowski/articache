package main

import (
	// provider "articache/internal/provider"
	"log"
	"net/http"
)

func main() {
	log.Println("Starting Articache")
	// TODO: Make cache path configurable
	cachePath = "/tmp"
	queue = make(chan artifactPath)
	httpClient = http.DefaultClient
	downloadLoop(20, queue)
	http.HandleFunc("/", HandleArtifactRequest)
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	http.ListenAndServe(":8080", nil)
}
