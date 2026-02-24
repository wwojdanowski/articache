package provider

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"articache/internal/metrics"
)

type Downloader interface {
	Download(ctx context.Context, rootPath string, ap artifactPath) error
}

type HTTPDownloader struct {
	httpClient *http.Client
}

type Cache struct {
	cachePath  string
	queue      chan artifactPath
	downloader Downloader
	mainRepo   string
}

func NewCache(path string, mainRepo string) *Cache {
	httpClient := &http.Client{Timeout: 2 * time.Minute}
	return NewCacheWithDownloader(path, mainRepo, &HTTPDownloader{httpClient: httpClient})
}

func NewCacheWithDownloader(cachePath string, mainRepo string, downloader Downloader) *Cache {
	// Buffered so request handling never blocks; we can drop on overflow.
	const queueSize = 1024
	return &Cache{
		cachePath:  cachePath,
		queue:      make(chan artifactPath, queueSize),
		downloader: downloader,
		mainRepo:   strings.TrimRight(mainRepo, "/"),
	}
}

func (c *Cache) Start(routines int) {
	c.downloadLoop(routines, c.queue)
}

func (c *Cache) cacheFilePath(requestPath string) (string, error) {
	// requestPath is a URL path like "/org/example/foo/1.0/foo-1.0.jar"
	// Convert to a filesystem path rooted under cachePath and prevent path traversal.
	cleanURLPath := path.Clean("/" + strings.TrimPrefix(requestPath, "/"))
	if cleanURLPath == "/" {
		return "", errors.New("empty artifact path")
	}
	rel := strings.TrimPrefix(cleanURLPath, "/")
	fullPath := filepath.Join(c.cachePath, filepath.FromSlash(rel))

	// Ensure fullPath stays within cachePath.
	relCheck, err := filepath.Rel(c.cachePath, fullPath)
	if err != nil {
		return "", fmt.Errorf("compute relative path: %w", err)
	}
	if relCheck == "." || strings.HasPrefix(relCheck, ".."+string(filepath.Separator)) || relCheck == ".." {
		return "", errors.New("invalid artifact path")
	}

	return fullPath, nil
}

func (c *Cache) findRequestedFile(requestPath string) (string, bool) {
	fullPath, err := c.cacheFilePath(requestPath)
	if err != nil {
		return "", false
	}
	if _, err := os.Stat(fullPath); err == nil {
		return fullPath, true
	}
	return "", false

}

func (d *HTTPDownloader) Download(ctx context.Context, rootPath string, ap artifactPath) error {
	downloadURL := strings.TrimRight(ap.repository, "/") + ap.name

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("download %q: %w", downloadURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// drain body (best effort) to allow connection reuse
		_, _ = io.Copy(io.Discard, resp.Body)
		return fmt.Errorf("download %q: unexpected status %d", downloadURL, resp.StatusCode)
	}

	filePath := filepath.Join(rootPath, filepath.FromSlash(strings.TrimPrefix(ap.name, "/")))
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %q: %w", dir, err)
	}

	tmp, err := os.CreateTemp(dir, filepath.Base(filePath)+".*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
	}()

	if _, err := io.Copy(tmp, resp.Body); err != nil {
		return fmt.Errorf("write %q: %w", tmpName, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close %q: %w", tmpName, err)
	}

	if err := os.Rename(tmpName, filePath); err != nil {
		return fmt.Errorf("rename %q -> %q: %w", tmpName, filePath, err)
	}

	slog.Info("artifact downloaded", "url", downloadURL)
	return nil
}

func (c *Cache) download(ctx context.Context, ap artifactPath) {
	start := time.Now()
	if err := c.downloader.Download(ctx, c.cachePath, ap); err != nil {
		metrics.DownloadsTotal.WithLabelValues("failure").Inc()
		metrics.DownloadDurationSeconds.Observe(time.Since(start).Seconds())
		slog.Error("artifact download failed", "artifact", ap.name, "repository", ap.repository, "error", err)
		return
	}
	metrics.DownloadsTotal.WithLabelValues("success").Inc()
	metrics.DownloadDurationSeconds.Observe(time.Since(start).Seconds())
}

type artifactPath struct {
	name       string
	repository string
}

func (c *Cache) downloadLoop(count int, queue <-chan artifactPath) {
	var mu sync.Mutex
	inflight := make(map[string]struct{})

	for i := 0; i < count; i++ {
		go func() {
			for val := range queue {
				metrics.DownloadQueueDepth.Set(float64(len(c.queue)))
				mu.Lock()
				if _, ok := inflight[val.name]; ok {
					mu.Unlock()
					continue
				}
				inflight[val.name] = struct{}{}
				mu.Unlock()

				metrics.DownloadsInflight.Inc()
				c.download(context.Background(), val)
				metrics.DownloadsInflight.Dec()

				mu.Lock()
				delete(inflight, val.name)
				mu.Unlock()
			}
		}()
	}
}

func (c *Cache) HandleArtifactRequest(w http.ResponseWriter, r *http.Request) {
	file := r.URL.Path
	start := time.Now()

	if _, err := c.cacheFilePath(file); err != nil {
		metrics.HTTPRequestsTotal.WithLabelValues("bad_request").Inc()
		http.Error(w, "invalid artifact path", http.StatusBadRequest)
		slog.Warn("invalid artifact request", "path", file, "remote_addr", r.RemoteAddr, "duration_ms", time.Since(start).Milliseconds())
		return
	}

	if filePath, ok := c.findRequestedFile(file); !ok {
		metrics.HTTPRequestsTotal.WithLabelValues("miss").Inc()
		metrics.CacheMissesTotal.Inc()
		repo := c.mainRepo
		alternatePath := repo + file
		http.Redirect(w, r, alternatePath, http.StatusSeeOther)
		select {
		case c.queue <- artifactPath{file, repo}:
			metrics.DownloadQueuedTotal.Inc()
			metrics.DownloadQueueDepth.Set(float64(len(c.queue)))
		default:
			metrics.DownloadQueueDroppedTotal.Inc()
			metrics.DownloadQueueDepth.Set(float64(len(c.queue)))
			slog.Warn("download queue full; skipping async download", "artifact", file)
		}
		slog.Info("artifact request", "result", "miss", "path", file, "status",
			http.StatusSeeOther, "remote_addr", r.RemoteAddr, "duration_ms",
			time.Since(start).Milliseconds())

	} else {
		metrics.HTTPRequestsTotal.WithLabelValues("hit").Inc()
		metrics.CacheHitsTotal.Inc()
		http.ServeFile(w, r, filePath)
		slog.Info("artifact request", "result", "hit", "path", file, "status",
			http.StatusOK, "remote_addr", r.RemoteAddr, "duration_ms",
			time.Since(start).Milliseconds())
	}

}
