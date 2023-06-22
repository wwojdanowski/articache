package main

import (
	// provider "articache/internal/provider"
	"log"
	"net/http"
)

func main() {
	log.Println("Starting Articache")
	// TODO: Make cache path configurable
	artifactProvider := &ArtifactProvider{CachePath: "/tmp", HttpClient: &http.Client{}}
	http.HandleFunc("/", artifactProvider.HandleArtifactRequest)
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	http.ListenAndServe(":8080", nil)
}
