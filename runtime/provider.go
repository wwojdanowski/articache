package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
)

var cachePath string
var queue chan artifactPath
var httpClient *http.Client

func findRequestedFile(file string) (string, bool) {
	fullPath := fmt.Sprintf("%s/%s", cachePath, file)
	if _, err := os.Stat(fullPath); err == nil {
		return fullPath, true
	}
	return "", false

}

func download(ap artifactPath) {
	url := fmt.Sprintf("%s%s", ap.repository, ap.name)
	resp, err := httpClient.Get(url)
	if err == nil {
		filePath := fmt.Sprintf("%s%s", cachePath, ap.name)
		tmpFilePath := fmt.Sprintf("%s.tmp", filePath)
		os.MkdirAll(path.Dir(tmpFilePath), 0750)
		out, _ := os.Create(tmpFilePath)
		defer out.Close()
		io.Copy(out, resp.Body)
		os.Rename(tmpFilePath, filePath)
		log.Printf("Successfully downloaded %s\n", url)
	} else {
		log.Printf("Issue when downloading %s, error: %s\n", url, err)

	}
	defer resp.Body.Close()
}

type artifactPath struct {
	name       string
	repository string
}

func downloadLoop(count int, queue <-chan artifactPath) {
	for i := 0; i < count; i++ {
		go func() {
			for val := range queue {
				download(val)
			}
		}()
	}
}

func HandleArtifactRequest(w http.ResponseWriter, r *http.Request) {
	u, err := url.Parse(r.URL.Path)
	if err != nil {
		panic(err)
	}
	file := u.Path
	log.Printf("Requesting artifact %s\n", file)

	if filePath, ok := findRequestedFile(file); !ok {
		log.Printf("Artifact %s not found in cache\n", file)
		repo := "https://repo.maven.apache.org/maven2"
		alternatePath := fmt.Sprintf("%s%s", repo, file)
		http.Redirect(w, r, alternatePath, http.StatusSeeOther)
		go func() {
			queue <- artifactPath{file, repo}
		}()

	} else {
		log.Printf("Found local artifact %s\n", file)
		http.ServeFile(w, r, filePath)
	}

}
