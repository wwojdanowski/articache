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

type FileRequestHandler struct {
	cachePath string
}

func (ap *ArtifactProvider) findRequestedFile(file string) (string, bool) {
	fullPath := fmt.Sprintf("%s/%s", ap.CachePath, file)
	if _, err := os.Stat(fullPath); err == nil {
		return fullPath, true
	}
	return "", false

}

type ArtifactProvider struct {
	CachePath  string
	HttpClient *http.Client
}

func (ap *ArtifactProvider) HandleArtifactRequest(w http.ResponseWriter, r *http.Request) {
	u, err := url.Parse(r.URL.Path)
	if err != nil {
		panic(err)
	}
	file := u.Path
	log.Printf("Requesting artifact %s\n", file)

	if filePath, ok := ap.findRequestedFile(file); !ok {
		alternatePath := fmt.Sprintf("https://repo.maven.apache.org/maven2%s", file)

		resp, err := ap.HttpClient.Get(alternatePath)
		if err != nil {
			log.Printf("Couldn't fetch file %s, err=%s", file, err)
			http.Redirect(w, r, alternatePath, http.StatusSeeOther)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			filePath := fmt.Sprintf("%s/%s", ap.CachePath, file)
			os.MkdirAll(path.Dir(filePath), 0750)
			out, _ := os.Create(filePath)
			defer out.Close()
			io.Copy(out, resp.Body)
			http.ServeFile(w, r, filePath)
		} else {
			log.Printf("Couldn't fetch file %s", file)
			http.NotFound(w, r)
		}

	} else {
		log.Printf("Found local artifact %s\n", file)
		http.ServeFile(w, r, filePath)
	}

}
