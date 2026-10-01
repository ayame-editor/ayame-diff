package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

// decodeStreamEvents runs a /api/diff/stream request and returns the NDJSON
// events in order. One JSON object per line is the wire contract the browser
// reads with a streaming fetch (#297).
func decodeStreamEvents(t *testing.T, body any) (*httptest.ResponseRecorder, []diffStreamEvent) {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	newTestServer(t).ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/diff/stream", bytes.NewReader(payload)))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if got := recorder.Header().Get("Content-Type"); !strings.Contains(got, "application/x-ndjson") {
		t.Fatalf("content-type=%q, want ndjson", got)
	}
	var events []diffStreamEvent
	for _, line := range strings.Split(strings.TrimSpace(recorder.Body.String()), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var event diffStreamEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatalf("decode event %q: %v", line, err)
		}
		events = append(events, event)
	}
	return recorder, events
}

func eventIndex(events []diffStreamEvent, match func(diffStreamEvent) bool) int {
	for index, event := range events {
		if match(event) {
			return index
		}
	}
	return -1
}

// TestDiffStreamReportsRealStages is the #297 contract: the stream names the
// phases the pipeline actually passes through, with a duration on each done,
// rather than an elapsed timer that says nothing about what is running.
func TestDiffStreamReportsRealStages(t *testing.T) {
	t.Parallel()
	body := map[string]any{
		"inline": true, "mode": "text",
		"oldText": "one\ntwo\nthree\nfour\n", "newText": "one\nTWO\nthree\nFOUR\n",
	}
	_, events := decodeStreamEvents(t, body)

	for _, want := range []struct{ stage, state string }{
		{"read", "active"}, {"read", "done"},
		{"compare", "active"}, {"compare", "done"},
		{"result", "active"}, {"result", "done"},
	} {
		if eventIndex(events, func(e diffStreamEvent) bool {
			return e.Type == "stage" && e.Stage == want.stage && e.State == want.state
		}) < 0 {
			t.Errorf("stream is missing stage %s/%s: %#v", want.stage, want.state, events)
		}
	}
	if eventIndex(events, func(e diffStreamEvent) bool { return e.Type == "done" }) < 0 {
		t.Fatalf("stream never ends: %#v", events)
	}
	// A finished phase reports how long it took, so the slow phase is visible.
	for _, event := range events {
		if event.Type == "stage" && event.State == "done" && event.ElapsedMs < 0 {
			t.Errorf("stage %s reported a negative duration", event.Stage)
		}
	}
}

// TestDiffStreamResultMatchesBufferedDiff keeps the two endpoints honest: the
// streamed pages must assemble to exactly what /api/diff returns.
func TestDiffStreamResultMatchesBufferedDiff(t *testing.T) {
	t.Parallel()
	body := map[string]any{
		"inline": true, "mode": "text",
		"oldText": "alpha\nbeta\ngamma\ndelta\n", "newText": "alpha\nBETA\ngamma\nDELTA\n",
	}
	_, events := decodeStreamEvents(t, body)

	var streamed *diffResponse
	for _, event := range events {
		if event.Type == "result" {
			streamed = event.Result
		}
	}
	if streamed == nil {
		t.Fatalf("no result event: %#v", events)
	}

	payload, _ := json.Marshal(body)
	recorder := httptest.NewRecorder()
	newTestServer(t).ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/diff", bytes.NewReader(payload)))
	var buffered diffResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &buffered); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(*streamed, buffered) {
		t.Fatalf("streamed result differs from buffered\nstreamed=%+v\nbuffered=%+v", *streamed, buffered)
	}
}

// TestDiffStreamPagesLargeResults covers the bounded first page: a result with
// more hunks than the page budget arrives as a first result plus result_page
// continuations, and the pages together are the whole hunk list.
func TestDiffStreamPagesLargeResults(t *testing.T) {
	t.Parallel()
	var old, neu strings.Builder
	for i := 0; i < diffStreamPageSize*2+5; i++ {
		// Each pair of lines is one changed block, separated by a fixed line so
		// window=1 keeps the hunks separate.
		old.WriteString("line-")
		old.WriteString(strings.Repeat("x", i%3))
		old.WriteString("\nkeep\n")
		neu.WriteString("LINE-")
		neu.WriteString(strings.Repeat("x", i%3))
		neu.WriteString("\nkeep\n")
	}
	body := map[string]any{
		"inline": true, "mode": "text",
		"oldText": old.String(), "newText": neu.String(),
		"window": 1, "maxHunks": 500,
	}
	_, events := decodeStreamEvents(t, body)

	var pages int
	var streamed []hunkOut
	for _, event := range events {
		switch event.Type {
		case "result":
			pages++
			if event.PageCount <= 1 {
				t.Fatalf("expected a paged result, got page_count=%d", event.PageCount)
			}
			streamed = append(streamed, event.Result.Hunks...)
		case "result_page":
			pages++
			streamed = append(streamed, event.Hunks...)
		}
	}
	if pages < 2 {
		t.Fatalf("large result was not paged: %d pages", pages)
	}
	if len(streamed) <= diffStreamPageSize {
		t.Fatalf("test did not produce more than one page of hunks: %d", len(streamed))
	}

	// The paged hunks equal the buffered response's hunks.
	payload, _ := json.Marshal(body)
	recorder := httptest.NewRecorder()
	newTestServer(t).ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/diff", bytes.NewReader(payload)))
	var buffered diffResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &buffered); err != nil {
		t.Fatal(err)
	}
	if len(streamed) != len(buffered.Hunks) {
		t.Fatalf("streamed %d hunks, buffered %d", len(streamed), len(buffered.Hunks))
	}
	for index := range buffered.Hunks {
		// The line text is capped per hunk; compare the geometry that decides
		// which hunk this is.
		if streamed[index].OldStart != buffered.Hunks[index].OldStart || streamed[index].NewStart != buffered.Hunks[index].NewStart {
			t.Fatalf("hunk %d differs: streamed=%+v buffered=%+v", index, streamed[index], buffered.Hunks[index])
		}
	}
}

// TestDiffStreamForecastsTruncationBeforeTheResult covers the degradation
// warning: when maxHunks cuts hunks, a forecast event arrives ahead of the
// result, so the reader is warned before meeting the gap rather than after.
func TestDiffStreamForecastsTruncationBeforeTheResult(t *testing.T) {
	t.Parallel()
	body := map[string]any{
		"inline": true, "mode": "text",
		"oldText": "a\nkeep\nb\nkeep\nc\nkeep\nd\nkeep\n",
		"newText": "A\nkeep\nB\nkeep\nC\nkeep\nD\nkeep\n",
		"window":  1, "maxHunks": 1,
	}
	_, events := decodeStreamEvents(t, body)

	forecastIndex := eventIndex(events, func(e diffStreamEvent) bool { return e.Type == "forecast" })
	resultIndex := eventIndex(events, func(e diffStreamEvent) bool { return e.Type == "result" })
	if forecastIndex < 0 {
		t.Fatalf("no truncation forecast: %#v", events)
	}
	if resultIndex < 0 || forecastIndex > resultIndex {
		t.Fatalf("forecast must precede the result: forecast=%d result=%d", forecastIndex, resultIndex)
	}
	forecast := events[forecastIndex]
	if !forecast.Degraded || forecast.Reason != "hunk_limit" {
		t.Fatalf("unexpected forecast: %+v", forecast)
	}
	if forecast.OmittedHunks == 0 || forecast.MaxHunks != 1 || forecast.HunkCount == 0 {
		t.Fatalf("forecast does not carry the real limits: %+v", forecast)
	}
}

// TestDiffStreamForecastsTruncatedLines covers the other degradation the engine
// knows about: maxLines cutting a long hunk's lines short.
func TestDiffStreamForecastsTruncatedLines(t *testing.T) {
	t.Parallel()
	// A long run of inserted lines forms one hunk longer than the cap, so the
	// response is a single hunk whose lines sliceLines cut short.
	var neu strings.Builder
	neu.WriteString("common\n")
	for i := 0; i < 40; i++ {
		neu.WriteString("added-")
		neu.WriteString(strings.Repeat("x", i%3))
		neu.WriteString("\n")
	}
	body := map[string]any{
		"inline": true, "mode": "text",
		"oldText": "common\n", "newText": neu.String(),
		"maxLines": 5,
	}
	_, events := decodeStreamEvents(t, body)
	forecastIndex := eventIndex(events, func(e diffStreamEvent) bool { return e.Type == "forecast" })
	if forecastIndex < 0 {
		t.Fatalf("no line-limit forecast: %#v", events)
	}
	forecast := events[forecastIndex]
	if forecast.Reason != "line_limit" || forecast.Truncated == 0 || forecast.MaxLines != 5 {
		t.Fatalf("unexpected line forecast: %+v", forecast)
	}
}

// TestDiffStreamMoveStageOnlyWhenAsked keeps the stage list honest: a phase the
// run does not perform is never announced.
func TestDiffStreamMoveStageOnlyWhenAsked(t *testing.T) {
	t.Parallel()
	body := map[string]any{
		"inline": true, "mode": "text",
		"oldText": "one\ntwo\nthree\nfour\n", "newText": "one\nTWO\nthree\nFOUR\n",
		"detectMoves": true,
	}
	_, events := decodeStreamEvents(t, body)
	if eventIndex(events, func(e diffStreamEvent) bool { return e.Type == "stage" && e.Stage == "moves" }) < 0 {
		t.Fatalf("detectMoves did not announce the moves stage: %#v", events)
	}

	_, withoutMoves := decodeStreamEvents(t, map[string]any{
		"inline": true, "mode": "text",
		"oldText": "one\ntwo\nthree\nfour\n", "newText": "one\nTWO\nthree\nFOUR\n",
	})
	if eventIndex(withoutMoves, func(e diffStreamEvent) bool { return e.Type == "stage" && e.Stage == "moves" }) >= 0 {
		t.Fatalf("a plain compare announced a moves stage: %#v", withoutMoves)
	}
}

func TestDiffStreamRequiresPost(t *testing.T) {
	t.Parallel()
	recorder := httptest.NewRecorder()
	newTestServer(t).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/diff/stream", nil))
	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d, want 405", recorder.Code)
	}
}
