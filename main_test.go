package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteError(t *testing.T) {
	w := httptest.NewRecorder()
	writeError(w, http.StatusInternalServerError, "db_insert_failed", "error generating shortcode", errors.New("connection refused"))
	if w.Code != http.StatusInternalServerError || w.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("unexpected response: %d %v", w.Code, w.Header())
	}
	var body ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Code != "db_insert_failed" || body.Error != "error generating shortcode" || body.Details != "connection refused" {
		t.Fatalf("unexpected body: %+v", body)
	}
}

func TestHandlersWithoutDatabase(t *testing.T) {
	for _, handler := range []http.HandlerFunc{CreateShortCode, ResolveShortCode} {
		w := httptest.NewRecorder()
		handler(w, httptest.NewRequest(http.MethodGet, "/", nil))
		if w.Code != http.StatusServiceUnavailable {
			t.Fatalf("got status %d", w.Code)
		}
		var body ErrorResponse
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if body.Code != "db_unavailable" || body.Details != "" {
			t.Fatalf("unexpected body: %+v", body)
		}
	}
}
