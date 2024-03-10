package main

import (
	"articache/internal/provider"
	"flag"
	"log"
	"net/http"
)

func main() {
	log.Println("Starting Articache")
	pathPtr := flag.String("path", "/tmp/articache_data", "Cache path.")
	repoPtr := flag.String("repo", "https://repo.maven.apache.org/maven2", "Main remote repository.")
	flag.Parse()
	cache := provider.NewCache(*pathPtr, *repoPtr)
	cache.Start(20)
	http.HandleFunc("/", cache.HandleArtifactRequest)
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	http.ListenAndServe(":8080", nil)
}
