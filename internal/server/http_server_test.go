package server

import (
	"net/http"
	"testing"
	"time"
)

func TestNewHTTPServerSetsDefensiveTimeouts(t *testing.T) {
	t.Parallel()
	srv := NewHTTPServer("127.0.0.1:0", http.NotFoundHandler())
	if srv.ReadHeaderTimeout != serverReadHeaderTimeout {
		t.Errorf("ReadHeaderTimeout=%v, want %v", srv.ReadHeaderTimeout, serverReadHeaderTimeout)
	}
	if srv.ReadTimeout != serverReadTimeout {
		t.Errorf("ReadTimeout=%v, want %v", srv.ReadTimeout, serverReadTimeout)
	}
	if srv.WriteTimeout != serverWriteTimeout {
		t.Errorf("WriteTimeout=%v, want %v", srv.WriteTimeout, serverWriteTimeout)
	}
	if srv.IdleTimeout != serverIdleTimeout {
		t.Errorf("IdleTimeout=%v, want %v", srv.IdleTimeout, serverIdleTimeout)
	}
	if srv.ReadHeaderTimeout <= 0 || srv.ReadTimeout <= 0 || srv.WriteTimeout <= 0 || srv.IdleTimeout <= 0 {
		t.Fatalf("all HTTP timeouts must be positive: %#v", srv)
	}
}

func TestLongOperationUsesSlidingWriteDeadline(t *testing.T) {
	t.Parallel()
	rec := &deadlineResponseWriter{header: make(http.Header)}
	w := longOperationResponseWriter(rec)
	if len(rec.writeDeadlines) != 1 || !rec.writeDeadlines[0].IsZero() {
		t.Fatalf("compute deadline=%v, want cleared deadline", rec.writeDeadlines)
	}
	w.WriteHeader(http.StatusOK)
	if len(rec.writeDeadlines) != 2 || !rec.writeDeadlines[1].After(time.Now()) {
		t.Fatalf("response header deadline=%v, want future deadline", rec.writeDeadlines)
	}
	if _, err := w.Write([]byte("response")); err != nil {
		t.Fatal(err)
	}
	if len(rec.writeDeadlines) != 3 || !rec.writeDeadlines[2].After(time.Now()) {
		t.Fatalf("renewed response deadline=%v, want future deadline", rec.writeDeadlines)
	}
}

func TestDropGetsBoundedExtendedDeadlines(t *testing.T) {
	t.Parallel()
	rec := &deadlineResponseWriter{header: make(http.Header)}
	before := time.Now()
	extendDropDeadlines(rec)
	if rec.readDeadline.Before(before.Add(dropTransferTimeout-time.Second)) ||
		rec.writeDeadlines[0].Before(before.Add(dropTransferTimeout-time.Second)) {
		t.Fatalf("drop deadlines read=%v write=%v, want about %v",
			rec.readDeadline, rec.writeDeadlines, dropTransferTimeout)
	}
}

type deadlineResponseWriter struct {
	header         http.Header
	readDeadline   time.Time
	writeDeadlines []time.Time
}

func (w *deadlineResponseWriter) Header() http.Header         { return w.header }
func (w *deadlineResponseWriter) WriteHeader(int)             {}
func (w *deadlineResponseWriter) Write(p []byte) (int, error) { return len(p), nil }
func (w *deadlineResponseWriter) SetReadDeadline(deadline time.Time) error {
	w.readDeadline = deadline
	return nil
}
func (w *deadlineResponseWriter) SetWriteDeadline(deadline time.Time) error {
	w.writeDeadlines = append(w.writeDeadlines, deadline)
	return nil
}
