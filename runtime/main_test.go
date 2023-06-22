package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

type MockTransport struct {
}

func (t *MockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	log.Println("MockTransport.RoundTrip called with ", req.URL.Path)
	switch req.URL.Path {
	case "/maven2/com.voovoo.recovered.jar":
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       ioutil.NopCloser(strings.NewReader("This is a recovered file")),
			Header:     nil,
		}, nil
	case "/maven2/com.voovoo.missing.jar":
		return &http.Response{
			StatusCode: http.StatusNotFound,
			Body:       nil,
			Header:     nil,
		}, nil
	case "/maven2/com.voovoo.error.jar":
		return &http.Response{
			StatusCode: http.StatusInternalServerError,
			Body:       nil,
			Header:     nil,
		}, fmt.Errorf("Internal Server Error")
	default:
		return &http.Response{
			StatusCode: http.StatusNotFound,
			Body:       nil,
			Header:     nil,
		}, nil
	}

}

var ap = ArtifactProvider{}

func setup() string {
	rootDir, err := ioutil.TempDir("/tmp", "articache_test_dir")

	if err != nil {
		fmt.Println("Error creating temporary directory:", err)
		os.Exit(1)
	}

	log.Println("Created temporary directory ", rootDir)

	filePath := rootDir + "/com.voovoo.existing.jar"
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY, 0644)

	if err != nil {
		fmt.Println("Error creating a test file:", err)
		os.Exit(1)
	}
	defer file.Close()

	file.WriteString("This is a test file")

	ap.CachePath = rootDir
	ap.HttpClient = &http.Client{Transport: &MockTransport{}}
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

func TestErrorRedirectsToMavenCentral(t *testing.T) {
	req, err := http.NewRequest("GET", "/com.voovoo.error.jar", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(ap.HandleArtifactRequest)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusSeeOther {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusSeeOther)
	}

	expected := `<a href="https://repo.maven.apache.org/maven2/com.voovoo.error.jar">See Other</a>.

`

	if rr.Body.String() != expected {
		t.Errorf("handler returned unexpected body: got %v want %v",
			rr.Body.String(), expected)
	}
}

func TestCachedArtifactIsDelivered(t *testing.T) {
	// given
	req, err := http.NewRequest("GET", "/com.voovoo.existing.jar", nil)
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(ap.HandleArtifactRequest)
	expectedBody := "This is a test file"

	// when
	handler.ServeHTTP(rr, req)

	// then
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	// and
	if rr.Body.String() != expectedBody {
		t.Errorf("handler returned unexpected body: got %v want %v",
			rr.Body.String(), expectedBody)
	}
}

func TestCacheMissDownloadsFileFromRepository(t *testing.T) {
	// given
	req, err := http.NewRequest("GET", "/com.voovoo.recovered.jar", nil)
	if err != nil {
		t.Fatal(err)
	}
	responseRecorder := httptest.NewRecorder()
	handler := http.HandlerFunc(ap.HandleArtifactRequest)
	expectedBody := "This is a recovered file"

	assert.NoFileExists(t, fmt.Sprintf("%s/com.voovoo.recovered.jar", ap.CachePath))

	// when
	handler.ServeHTTP(responseRecorder, req)

	// then
	assert.Equal(t, http.StatusOK, responseRecorder.Code, "handler returned wrong status code")
	assert.Equal(t, expectedBody, responseRecorder.Body.String(), "handler returned unexpected body")

	// and
	assert.FileExists(t, fmt.Sprintf("%s/com.voovoo.recovered.jar", ap.CachePath))
}

func TestUnknownFile(t *testing.T) {
	// given
	req, err := http.NewRequest("GET", "/com.voovoo.unknown.jar", nil)
	if err != nil {
		t.Fatal(err)
	}
	responseRecorder := httptest.NewRecorder()
	handler := http.HandlerFunc(ap.HandleArtifactRequest)
	expectedBody := "404 page not found\n"

	assert.NoFileExists(t, fmt.Sprintf("%s/com.voovoo.unknown.jar", ap.CachePath))

	// when
	handler.ServeHTTP(responseRecorder, req)

	// then
	assert.Equal(t, http.StatusNotFound, responseRecorder.Code, "handler returned wrong status code")
	assert.Equal(t, expectedBody, responseRecorder.Body.String(), "handler returned unexpected body")

	// and
	assert.NoFileExists(t, fmt.Sprintf("%s/com.voovoo.unknown.jar", ap.CachePath))
}
