package main_test

import (
	"articache/internal/provider"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func setup() string {
	rootDir, err := ioutil.TempDir("/tmp", "articache_test_dir")

	if err != nil {
		fmt.Println("Error creating temporary directory:", err)
		os.Exit(1)
	}

	log.Println("Created temporary directory ", rootDir)

	return rootDir

}

func teardown(rootDir string) {
	log.Println("Removing temporary directory ", rootDir)

	if err := os.RemoveAll(rootDir); err != nil {
		fmt.Println("Error removing temporary directory:", err)
		os.Exit(1)
	}
}

func TestMain(m *testing.M) {
	rootDir := setup()

	code := m.Run()
	teardown(rootDir)
	os.Exit(code)
}

func setupSuite(t *testing.T) string {
	return setup()
}

func teardownSuite(t *testing.T, dir string) {
	teardown(dir)
}

type fakeMavenRepo struct {
	server *http.Server
	url    string
}

func setupFakeMavenRepo() fakeMavenRepo {
	mux := http.NewServeMux()
	mux.HandleFunc("/maven2/", func(rw http.ResponseWriter, r *http.Request) {
		rw.Write([]byte{0, 0, 0, 0})
		rw.WriteHeader(http.StatusOK)
	})
	server := &http.Server{Addr: ":8081", Handler: mux}
	go func() {
		if err := server.ListenAndServe(); err != nil {
			log.Printf("Httpserver: ListenAndServe() error: %s", err)
		}
	}()

	return fakeMavenRepo{server, "http://127.0.0.1:8081/maven2"}
}

func teardownFakeMavenRepo(repo fakeMavenRepo) {
	repo.server.Close()
}

func TestRedirect(t *testing.T) {

	// setup http server
	mux := http.NewServeMux()
	mux.HandleFunc("/maven2/", func(rw http.ResponseWriter, r *http.Request) {
		rw.Write([]byte{0, 0, 0, 0})
		rw.WriteHeader(http.StatusOK)
	})
	repo := setupFakeMavenRepo()
	defer teardownFakeMavenRepo(repo)

	rootDir := setupSuite(t)
	defer teardownSuite(t, rootDir)

	cache := provider.NewCache(rootDir, repo.url)
	cache.Start(20)
	artifacts := []string{"/com.voovoo.lib.jar", "/com.booboo.lib.jar", "/com.noonoo.lib.jar"}

	http.HandleFunc("/", cache.HandleArtifactRequest)
	go func() {
		if err := http.ListenAndServe(":8080", nil); err != nil {
			log.Printf("Failed to start cache server")
		}
	}()

	for i := range artifacts {
		response, _ := http.Get(fmt.Sprintf("http://127.0.0.1:8080%s", artifacts[i]))
		assert.Equal(t, http.StatusOK, response.StatusCode)
	}

	time.Sleep(time.Second)
	for i := range artifacts {
		file := fmt.Sprintf("%s%s", rootDir, artifacts[i])
		assert.FileExists(t, file)
	}
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
