package mcp

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRemoteRejectsMissingCredential(t *testing.T) {
	handler := New(nil, nil).Handler(nil)
	for _, header := range []string{"", "Bearer invalid", "Basic anything"} {
		request := httptest.NewRequest(http.MethodPost, "http://localhost/mcp", nil)
		request.Header.Set("Authorization", header)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("got %d", response.Code)
		}
	}
}
func TestRemoteRejectsForeignOrigin(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "http://localhost/mcp", nil)
	request.Header.Set("Origin", "https://foreign.example")
	response := httptest.NewRecorder()
	New(nil, nil).Handler(nil).ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("got %d", response.Code)
	}
}
