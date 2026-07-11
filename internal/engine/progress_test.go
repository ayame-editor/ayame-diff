package engine

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestProgressCounterWritesInjectedLogAndEvent(t *testing.T) {
	t.Parallel()
	var log bytes.Buffer
	var events []ProgressEvent
	p := startProgress(context.Background(), "partition", "left", false, &log, func(event ProgressEvent) {
		events = append(events, event)
	})
	p.add(7, 2*1024*1024)
	p.print(true)
	if len(events) != 1 {
		t.Fatalf("events = %d, want 1", len(events))
	}
	event := events[0]
	if event.Phase != "partition" || event.Label != "left" || !event.Done || event.Rows != 7 || event.Bytes != 2*1024*1024 {
		t.Fatalf("event = %#v", event)
	}
	if got := log.String(); !strings.Contains(got, "stage done: partition left") || !strings.Contains(got, "2.0MiB") {
		t.Fatalf("log = %q", got)
	}
}

func TestProgressCounterCanUseCallbackWithoutLog(t *testing.T) {
	t.Parallel()
	called := false
	p := startProgress(context.Background(), "assemble", "", false, nil, func(event ProgressEvent) {
		called = event.Done
	})
	p.print(true)
	if !called {
		t.Fatal("callback was not invoked")
	}
}
