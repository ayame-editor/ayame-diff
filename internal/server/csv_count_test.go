package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCSVDifferenceCountDedupesInConstantMemory is the #156 regression, and it
// pins an assumption the handler cannot check for itself.
//
// Counting distinct differences used to accumulate every ID in a set that
// nothing bounded — a comparison of two large divergent files sized it by the
// input. The set was replaced by a comparison against the previous ID, which is
// exact only because the engine emits identical rows consecutively: records are
// sorted within a key group and a key group never spans partitions.
//
// The input below scatters duplicates so that a naive read of the file would
// see them interleaved. If the engine ever stopped grouping them, this test
// fails rather than the count silently inflating.
func TestCSVDifferenceCountDedupesInConstantMemory(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	left, right := filepath.Join(dir, "left.csv"), filepath.Join(dir, "right.csv")

	// Key K holds the same row three times, interleaved with a different row of
	// the same key: A, B, A, B, A. Distinct differences under K: 2.
	var builder strings.Builder
	builder.WriteString("id,v\n")
	for i := range 3 {
		builder.WriteString("K,A\n")
		if i < 2 {
			builder.WriteString("K,B\n")
		}
	}
	// A second duplicated group, far away in the file, plus a matching row.
	for range 4 {
		builder.WriteString("M,dup\n")
	}
	builder.WriteString("Z,same\n")
	if err := os.WriteFile(left, []byte(builder.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(right, []byte("id,v\nZ,same\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	body := map[string]any{
		"old": left, "new": right, "keyNames": []string{"id"}, "keyMode": "include",
		"hasHeader": true,
	}
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/csv/diff", strings.NewReader(string(payload)))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	newTestServer(t).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		DifferenceCount int `json:"difference_count"`
		Differences     []struct {
			ID string `json:"id"`
		} `json:"differences"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode: %v (%s)", err, recorder.Body.String())
	}

	// Distinct IDs actually returned are the ground truth for what the count
	// should be, computed here with the set the handler no longer keeps.
	distinct := map[string]struct{}{}
	for _, difference := range response.Differences {
		distinct[difference.ID] = struct{}{}
	}
	if response.DifferenceCount != len(distinct) {
		t.Fatalf("difference_count=%d but %d distinct IDs were returned — consecutive dedupe is over- or under-counting",
			response.DifferenceCount, len(distinct))
	}

	// And the emission order itself: every run of one ID must be contiguous.
	seen := map[string]int{}
	previous := ""
	for _, difference := range response.Differences {
		if difference.ID != previous {
			seen[difference.ID]++
			previous = difference.ID
		}
	}
	for id, runs := range seen {
		if runs > 1 {
			t.Errorf("ID %s appears in %d separate runs; the engine no longer groups identical rows, "+
				"so counting against the previous ID is no longer exact", id, runs)
		}
	}
	if len(distinct) < 2 {
		t.Fatalf("fixture produced %d distinct differences; it must exercise duplicates", len(distinct))
	}
}

// TestCSVDifferenceCountSurvivesTruncation checks the case that made the old
// set unbounded: the loop keeps counting after it stops collecting rows, so the
// count must stay correct past maxRows while memory does not grow.
func TestCSVDifferenceCountSurvivesTruncation(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	left, right := filepath.Join(dir, "left.csv"), filepath.Join(dir, "right.csv")
	var builder strings.Builder
	builder.WriteString("id,v\n")
	const rows = 60
	for i := range rows {
		fmt.Fprintf(&builder, "k%03d,value%d\n", i, i)
	}
	if err := os.WriteFile(left, []byte(builder.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(right, []byte("id,v\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(map[string]any{
		"old": left, "new": right, "keyNames": []string{"id"}, "keyMode": "include",
		"hasHeader": true, "maxRows": 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/csv/diff", strings.NewReader(string(payload)))
	recorder := httptest.NewRecorder()
	newTestServer(t).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		DifferenceCount int  `json:"difference_count"`
		Truncated       bool `json:"truncated"`
		Differences     []struct {
			ID string `json:"id"`
		} `json:"differences"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !response.Truncated {
		t.Fatal("fixture did not exceed maxRows, so truncation is untested")
	}
	if len(response.Differences) > 10 {
		t.Errorf("returned %d rows despite maxRows=10", len(response.Differences))
	}
	if response.DifferenceCount != rows {
		t.Errorf("difference_count=%d, want %d — counting must continue past truncation", response.DifferenceCount, rows)
	}
}
