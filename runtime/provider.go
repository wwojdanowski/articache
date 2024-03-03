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

type Cache struct {
	cachePath  string
	queue      chan artifactPath
	httpClient *http.Client
}

func (c *Cache) findRequestedFile(file string) (string, bool) {
	fullPath := fmt.Sprintf("%s/%s", c.cachePath, file)
	if _, err := os.Stat(fullPath); err == nil {
		return fullPath, true
	}
	return "", false

}

func (c *Cache) download(ap artifactPath) {
	url := fmt.Sprintf("%s%s", ap.repository, ap.name)
	resp, err := c.httpClient.Get(url)
	if err == nil {
		filePath := fmt.Sprintf("%s%s", c.cachePath, ap.name)
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

func (c *Cache) downloadLoop(count int, queue <-chan artifactPath) {
	mutex := make(chan struct{}, 1)
	lock := make(map[string]bool)

	for i := 0; i < count; i++ {
		go func() {
			for val := range queue {
				mutex <- struct{}{}
				if _, ok := lock[val.name]; ok {
					<-mutex
					continue
				} else {
					lock[val.name] = true
					<-mutex
				}

				c.download(val)

				mutex <- struct{}{}
				delete(lock, val.name)
				<-mutex
			}
		}()
	}
}

func (c *Cache) HandleArtifactRequest(w http.ResponseWriter, r *http.Request) {
	u, err := url.Parse(r.URL.Path)
	if err != nil {
		// TODO: shouldn't panic
		panic(err)
	}
	file := u.Path
	log.Printf("Requesting artifact %s\n", file)

	if filePath, ok := c.findRequestedFile(file); !ok {
		log.Printf("Artifact %s not found in cache\n", file)
		repo := "https://repo.maven.apache.org/maven2"
		alternatePath := fmt.Sprintf("%s%s", repo, file)
		http.Redirect(w, r, alternatePath, http.StatusSeeOther)
		go func() {
			c.queue <- artifactPath{file, repo}
		}()

	} else {
		log.Printf("Found local artifact %s\n", file)
		http.ServeFile(w, r, filePath)
	}

}
