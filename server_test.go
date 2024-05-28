// main_test.go
package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCaptureResponseWriter(t *testing.T) {
	dataChan := make(chan []byte, 10)
	recorder := httptest.NewRecorder()
	writer := &captureResponseWriter{
		ResponseWriter: recorder,
		DataChan:       dataChan,
	}

	testData := "test data"
	writer.Write([]byte(testData))

	select {
	case data := <-dataChan:
		if string(data) != testData {
			t.Errorf("Expected %s, got %s", testData, data)
		}
	default:
		t.Error("Expected data in channel, but got none")
	}
}

func TestSSEResponseStreamingMiddleware(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hello, World!"))
	})

	middleware := sseResponseStreamingMiddleware(handler)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Use-SSE", "true")
	recorder := httptest.NewRecorder()

	middleware.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, recorder.Code)
	}

	var sessionData SessionMetadata
	err := json.Unmarshal(recorder.Body.Bytes(), &sessionData)
	if err != nil {
		t.Errorf("Failed to unmarshal session data: %v", err)
	}

	if sessionData.SessionUUID == "" {
		t.Error("Expected non-empty session UUID")
	}
}

func TestSSEHandler(t *testing.T) {
	sessionUUID := generateSecureSessionID()
	dataChan := make(chan []byte, 10)
	dataChanMap.Store(sessionUUID, dataChan)
	defer dataChanMap.Delete(sessionUUID)

	server := httptest.NewServer(http.HandlerFunc(sseHandler))
	defer server.Close()

	go func() {
		dataChan <- []byte("test data")
		close(dataChan)
	}()

	resp, err := http.Get(fmt.Sprintf("%s/sse/%s", server.URL, sessionUUID))
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, resp.StatusCode)
	}

	expectedData := "data: test data\n\n"
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}
	body := string(bodyBytes)

	if !strings.Contains(body, expectedData) {
		t.Errorf("Expected response to contain %s, got %s", expectedData, body)
	}
}
