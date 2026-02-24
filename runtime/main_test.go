package main_test

import (
	"articache/internal/metrics"
	"articache/internal/provider"
	"fmt"
	"io"
	"log"
	"net/http"
	"testing"
	"time"

	"net/http/httptest"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/stretchr/testify/assert"
)

func TestRedirect(t *testing.T) {
	repo := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			rw.WriteHeader(http.StatusOK)
			return
		}
		// emulate Maven Central content
		rw.WriteHeader(http.StatusOK)
		_, _ = rw.Write([]byte{0, 0, 0, 0})
	}))
	defer repo.Close()

	rootDir := t.TempDir()
	log.Println("Created temporary directory ", rootDir)

	cache := provider.NewCache(rootDir, repo.URL+"/maven2")
	cache.Start(20)

	cacheServer := httptest.NewServer(http.HandlerFunc(cache.HandleArtifactRequest))
	defer cacheServer.Close()

	artifacts := []string{"/com.voovoo.lib.jar", "/com.booboo.lib.jar", "/com.noonoo.lib.jar"}

	for i := range artifacts {
		if response, err := http.Get(fmt.Sprintf("%s%s", cacheServer.URL, artifacts[i])); err == nil {
			assert.Equal(t, http.StatusOK, response.StatusCode)
		} else {
			assert.Fail(t, fmt.Sprintf("An error occured: %s", err))
		}
	}

	time.Sleep(time.Second)
	for i := range artifacts {
		file := fmt.Sprintf("%s%s", rootDir, artifacts[i])
		assert.FileExists(t, file)
	}
}

func TestMetricsEndpoint(t *testing.T) {
	rootDir := t.TempDir()

	reg := prometheus.NewRegistry()
	metrics.Register(reg)

	repo := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		rw.WriteHeader(http.StatusOK)
	}))
	defer repo.Close()

	cache := provider.NewCache(rootDir, repo.URL+"/maven2")
	cache.Start(2)

	mux := http.NewServeMux()
	mux.HandleFunc("/", cache.HandleArtifactRequest)
	mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))

	srv := httptest.NewServer(mux)
	defer srv.Close()

	// trigger at least one request so our custom metrics are initialized
	respReq, err := http.Get(srv.URL + "/some-artifact.jar")
	assert.NoError(t, err)
	if respReq != nil {
		respReq.Body.Close()
	}

	resp, err := http.Get(srv.URL + "/metrics")
	assert.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)
	assert.Contains(t, string(body), "articache_http_requests_total")
}

// func TestCachedArtifactIsDelivered(t *testing.T) {

// 	repo := "https://repo.maven.apache.org/maven2"
// 	cache := Cache{"/tmp", make(chan artifactPath, 10), &MockDownloader{}}
// 	artifacts := []string{"/com.voovoo.lib.jar", "/com.booboo.lib.jar", "/com.noonoo.lib.jar"}
// 	expected := []artifactPath{
// 		{name: "/com.voovoo.lib.jar", repository: repo},
// 		{name: "/com.booboo.lib.jar", repository: repo},
// 		{name: "/com.noonoo.lib.jar", repository: repo},
// 	}

// 	rr := httptest.NewRecorder()
// 	handler := http.HandlerFunc(cache.HandleArtifactRequest)

// 	for _, a := range artifacts {
// 		req, err := http.NewRequest("GET", a, nil)
// 		handler.ServeHTTP(rr, req)
// 		if err != nil {
// 			t.Fatal(err)
// 		}
// 		if status := rr.Code; status != http.StatusSeeOther {
// 			t.Errorf("handler returned wrong status code: got %v want %v",
// 				status, http.StatusSeeOther)
// 		}
// 	}

// 	out := make([]artifactPath, 0)
// 	for i := 0; i < 3; i++ {
// 		out = append(out, <-cache.queue)
// 	}
// 	assert.Len(t, out, 3)
// 	assert.ElementsMatch(t, expected, out)

// }

// type MockTransport struct {
// }

// func (t *MockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
// 	log.Println("MockTransport.RoundTrip called with ", req.URL.Path)
// 	switch req.URL.Path {
// 	case "/maven2/com.voovoo.recovered.jar":
// 		return &http.Response{
// 			StatusCode: http.StatusOK,
// 			Body:       ioutil.NopCloser(strings.NewReader("This is a recovered file")),
// 			Header:     nil,
// 		}, nil
// 	case "/maven2/com.voovoo.missing.jar":
// 		return &http.Response{
// 			StatusCode: http.StatusNotFound,
// 			Body:       nil,
// 			Header:     nil,
// 		}, nil
// 	case "/maven2/com.voovoo.error.jar":
// 		return &http.Response{
// 			StatusCode: http.StatusInternalServerError,
// 			Body:       nil,
// 			Header:     nil,
// 		}, fmt.Errorf("Internal Server Error")
// 	default:
// 		return &http.Response{
// 			StatusCode: http.StatusNotFound,
// 			Body:       nil,
// 			Header:     nil,
// 		}, nil
// 	}

// }

// func setup() string {
// 	rootDir, err := ioutil.TempDir("/tmp", "articache_test_dir")

// 	if err != nil {
// 		fmt.Println("Error creating temporary directory:", err)
// 		os.Exit(1)
// 	}

// 	log.Println("Created temporary directory ", rootDir)

// 	filePath := rootDir + "/com.voovoo.existing.jar"
// 	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY, 0644)

// 	if err != nil {
// 		fmt.Println("Error creating a test file:", err)
// 		os.Exit(1)
// 	}
// 	defer file.Close()

// 	file.WriteString("This is a test file")

// 	cachePath = rootDir
// 	httpClient = &http.Client{Transport: &MockTransport{}}
// 	return rootDir

// }

// func teardown(rootDir string) {
// 	log.Println("Removing temporary directory ", rootDir)

// 	if err := os.RemoveAll(rootDir); err != nil {
// 		fmt.Println("Error removing temporary directory:", err)
// 		os.Exit(1)
// 	}
// }

// func TestMain(m *testing.M) {
// 	rootDir := setup()
// 	code := m.Run()
// 	teardown(rootDir)
// 	os.Exit(code)
// }

// func TestErrorRedirectsToMavenCentral(t *testing.T) {
// 	req, err := http.NewRequest("GET", "/com.voovoo.error.jar", nil)
// 	if err != nil {
// 		t.Fatal(err)
// 	}

// 	rr := httptest.NewRecorder()
// 	handler := http.HandlerFunc(HandleArtifactRequest)

// 	handler.ServeHTTP(rr, req)

// 	if status := rr.Code; status != http.StatusSeeOther {
// 		t.Errorf("handler returned wrong status code: got %v want %v",
// 			status, http.StatusSeeOther)
// 	}

// 	expected := `<a href="https://repo.maven.apache.org/maven2/com.voovoo.error.jar">See Other</a>.

// `

// 	if rr.Body.String() != expected {
// 		t.Errorf("handler returned unexpected body: got %v want %v",
// 			rr.Body.String(), expected)
// 	}
// }

// func TestCachedArtifactIsDelivered(t *testing.T) {
// 	// given
// 	req, err := http.NewRequest("GET", "/com.voovoo.existing.jar", nil)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	rr := httptest.NewRecorder()
// 	handler := http.HandlerFunc(HandleArtifactRequest)
// 	expectedBody := "This is a test file"

// 	// when
// 	handler.ServeHTTP(rr, req)

// 	// then
// 	if status := rr.Code; status != http.StatusOK {
// 		t.Errorf("handler returned wrong status code: got %v want %v",
// 			status, http.StatusOK)
// 	}

// 	// and
// 	if rr.Body.String() != expectedBody {
// 		t.Errorf("handler returned unexpected body: got %v want %v",
// 			rr.Body.String(), expectedBody)
// 	}
// }

// func TestCacheMissDownloadsFileFromRepository(t *testing.T) {
// 	// given
// 	req, err := http.NewRequest("GET", "/com.voovoo.recovered.jar", nil)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	responseRecorder := httptest.NewRecorder()
// 	handler := http.HandlerFunc(HandleArtifactRequest)
// 	expectedBody := "This is a recovered file"

// 	assert.NoFileExists(t, fmt.Sprintf("%s/com.voovoo.recovered.jar", cachePath))

// 	// when
// 	handler.ServeHTTP(responseRecorder, req)

// 	// then
// 	assert.Equal(t, http.StatusOK, responseRecorder.Code, "handler returned wrong status code")
// 	assert.Equal(t, expectedBody, responseRecorder.Body.String(), "handler returned unexpected body")

// 	// and
// 	assert.FileExists(t, fmt.Sprintf("%s/com.voovoo.recovered.jar", cachePath))
// }

// func TestUnknownFile(t *testing.T) {
// 	// given
// 	req, err := http.NewRequest("GET", "/com.voovoo.unknown.jar", nil)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	responseRecorder := httptest.NewRecorder()
// 	handler := http.HandlerFunc(HandleArtifactRequest)
// 	expectedBody := "404 page not found\n"

// 	assert.NoFileExists(t, fmt.Sprintf("%s/com.voovoo.unknown.jar", cachePath))

// 	// when
// 	handler.ServeHTTP(responseRecorder, req)

// 	// then
// 	assert.Equal(t, http.StatusNotFound, responseRecorder.Code, "handler returned wrong status code")
// 	assert.Equal(t, expectedBody, responseRecorder.Body.String(), "handler returned unexpected body")

// 	// and
// 	assert.NoFileExists(t, fmt.Sprintf("%s/com.voovoo.unknown.jar", cachePath))
// }
