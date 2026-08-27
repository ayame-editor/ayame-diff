package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func postDiffContext(t *testing.T, body any) *httptest.ResponseRecorder {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	newTestServer(t).ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/diff/context", bytes.NewReader(payload)))
	return recorder
}

func TestDiffContextReturnsOnlyRequestedInlineRanges(t *testing.T) {
	recorder := postDiffContext(t, map[string]any{
		"inline":  true,
		"oldText": "zero\none\ntwo\nthree\nfour\n",
		"newText": "ZERO\nONE\nTWO\nTHREE\nFOUR\n",
		"mode":    "text",
		"ranges": []map[string]any{
			{"id": 17, "old_start": 1, "new_start": 2, "count": 2},
			{"id": 23, "old_start": 4, "new_start": 0, "count": 1},
		},
	})
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var response diffContextResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Ranges) != 2 {
		t.Fatalf("ranges=%#v", response.Ranges)
	}
	if got, want := strings.Join(response.Ranges[0].Old, ","), "one,two"; got != want {
		t.Fatalf("old range=%q want %q", got, want)
	}
	if got, want := strings.Join(response.Ranges[0].New, ","), "TWO,THREE"; got != want {
		t.Fatalf("new range=%q want %q", got, want)
	}
	if response.Ranges[0].ID != 17 || response.Ranges[1].ID != 23 {
		t.Fatalf("range IDs were not preserved: %#v", response.Ranges)
	}
}

func TestDiffContextRejectsUnboundedAndOutOfBoundsRanges(t *testing.T) {
	tests := []struct {
		name   string
		ranges any
		want   string
	}{
		{name: "empty", ranges: []any{}, want: "ranges must contain"},
		{name: "zero", ranges: []map[string]any{{"old_start": 0, "new_start": 0, "count": 0}}, want: "range count"},
		{name: "too large", ranges: []map[string]any{{"old_start": 0, "new_start": 0, "count": maxDiffContextLines + 1}}, want: "range count"},
		{name: "outside", ranges: []map[string]any{{"old_start": 2, "new_start": 0, "count": 1}}, want: "outside"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := postDiffContext(t, map[string]any{
				"inline": true, "oldText": "one\n", "newText": "one\n", "mode": "text", "ranges": test.ranges,
			})
			if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), test.want) {
				t.Fatalf("status=%d body=%s, want %q", recorder.Code, recorder.Body.String(), test.want)
			}
		})
	}
}

func TestDiffContextRequiresPost(t *testing.T) {
	recorder := httptest.NewRecorder()
	newTestServer(t).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/diff/context", nil))
	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d", recorder.Code)
	}
}
