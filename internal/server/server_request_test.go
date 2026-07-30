package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDecodePostJSON(t *testing.T) {
	t.Parallel()

	type request struct {
		Name string `json:"name"`
	}

	t.Run("rejects_non_post", func(t *testing.T) {
		t.Parallel()
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		if _, ok := decodePostJSON[request](rec, req, ""); ok {
			t.Fatal("decodePostJSON accepted GET")
		}
		if rec.Code != http.StatusMethodNotAllowed || !strings.Contains(rec.Body.String(), "use POST") {
			t.Fatalf("response = %d %q", rec.Code, rec.Body.String())
		}
	})

	t.Run("decodes_request", func(t *testing.T) {
		t.Parallel()
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"ayame"}`))
		got, ok := decodePostJSON[request](rec, req, "")
		if !ok || got.Name != "ayame" {
			t.Fatalf("request = %+v, ok = %v", got, ok)
		}
	})

	t.Run("reports_decoder_error", func(t *testing.T) {
		t.Parallel()
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("{"))
		if _, ok := decodePostJSON[request](rec, req, ""); ok {
			t.Fatal("decodePostJSON accepted invalid JSON")
		}
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "invalid JSON:") {
			t.Fatalf("response = %d %q", rec.Code, rec.Body.String())
		}
	})

	t.Run("keeps_domain_error", func(t *testing.T) {
		t.Parallel()
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("{"))
		if _, ok := decodePostJSON[request](rec, req, "project path is required"); ok {
			t.Fatal("decodePostJSON accepted invalid JSON")
		}
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "project path is required") {
			t.Fatalf("response = %d %q", rec.Code, rec.Body.String())
		}
	})
}
