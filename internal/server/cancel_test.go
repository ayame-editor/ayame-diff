package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestDiffHandlerHonorsCanceledContext covers #169: when the request context is
// cancelled (the browser disconnected), the diff handler aborts instead of
// running the full comparison to completion and replying 200.
func TestDiffHandlerHonorsCanceledContext(t *testing.T) {
	t.Parallel()
	var b strings.Builder
	for i := 0; i < 3000; i++ {
		b.WriteString("line\n")
	}
	text := b.String()
	body, _ := json.Marshal(map[string]any{"inline": true, "oldText": text, "newText": text, "mode": "text"})

	req := httptest.NewRequest(http.MethodPost, "/api/diff", bytes.NewReader(body))
	ctx, cancel := context.WithCancel(req.Context())
	cancel()
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	newTestServer(t).ServeHTTP(rec, req)
	if rec.Code == http.StatusOK {
		t.Fatalf("cancelled request produced a full 200 diff; want an abort status. body=%s", rec.Body)
	}
}
