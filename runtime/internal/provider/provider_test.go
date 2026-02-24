package provider

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type MockDownloader struct {
	mu        sync.Mutex
	Downloads map[string]int
}

func (d *MockDownloader) Download(ctx context.Context, rootPath string, ap artifactPath) error {
	d.mu.Lock()
	d.Downloads[ap.name] = d.Downloads[ap.name] + 1
	d.mu.Unlock()
	time.Sleep(time.Second * 3)
	return nil
}

func TestDownloadLoopSingleFile(t *testing.T) {
	repo := "https://repo.maven.apache.org/maven2"
	artifact := artifactPath{name: "/com.voovoo.lib.jar", repository: repo}
	downloader := MockDownloader{Downloads: make(map[string]int)}
	cache := Cache{"/tmp", make(chan artifactPath, 10), &downloader, repo}
	cache.downloadLoop(3, cache.queue)

	for i := 0; i < 5; i++ {
		cache.queue <- artifact
	}

	time.Sleep(time.Second * 1)
	assert.Equal(t, 1, downloader.Downloads["/com.voovoo.lib.jar"])
}

func TestDownloadLoopMultipleFiles(t *testing.T) {
	repo := "https://repo.maven.apache.org/maven2"
	artifacts := []artifactPath{
		{name: "/com.voovoo.lib.jar", repository: repo},
		{name: "/com.booboo.lib.jar", repository: repo},
		{name: "/com.noonoo.lib.jar", repository: repo},
	}
	downloader := MockDownloader{Downloads: make(map[string]int)}
	cache := Cache{"/tmp", make(chan artifactPath, 10), &downloader, repo}
	cache.downloadLoop(3, cache.queue)

	for i := range artifacts {
		cache.queue <- artifacts[i]
	}

	time.Sleep(time.Second * 1)

	for i := range artifacts {
		assert.Equal(t, 1, downloader.Downloads[artifacts[i].name])
	}

}
