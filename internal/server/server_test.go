package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandlePing(t *testing.T) {
	// Create a minimal server instance since HandlePing doesn't use any fields
	srv := &Server{}

	// Create a request
	req := httptest.NewRequest("GET", "/ping", nil)
	w := httptest.NewRecorder()

	// Call the handler
	srv.HandlePing(w, req)

	// Check the status code
	if status := w.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	// Check the response body
	expected := "pong"
	if w.Body.String() != expected {
		t.Errorf("handler returned unexpected body: got %v want %v",
			w.Body.String(), expected)
	}

	// Check the headers
	if origin := w.Header().Get("Access-Control-Allow-Origin"); origin != "*" {
		t.Errorf("handler returned wrong Access-Control-Allow-Origin: got %v want %v",
			origin, "*")
	}

	if contentType := w.Header().Get("Content-Type"); contentType != "text/plain" {
		t.Errorf("handler returned wrong Content-Type: got %v want %v",
			contentType, "text/plain")
	}
}
