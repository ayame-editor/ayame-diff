package server

import (
	"net/http"
	"time"
)

// Network-facing timeouts protect the local GUI from slow headers, bodies,
// stalled writes, and abandoned keep-alive connections (#109). Ordinary API
// request bodies are small JSON documents; browser file drops and expensive
// comparison responses get the scoped exceptions below.
const (
	serverReadHeaderTimeout = 10 * time.Second
	serverReadTimeout       = 30 * time.Second
	serverWriteTimeout      = 30 * time.Second
	serverIdleTimeout       = 2 * time.Minute
	dropTransferTimeout     = time.Hour
)

// NewHTTPServer applies the same hardened transport settings to `serve` and
// `gui`. Callers may use ListenAndServe or pass an existing listener to Serve.
func NewHTTPServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: serverReadHeaderTimeout,
		ReadTimeout:       serverReadTimeout,
		WriteTimeout:      serverWriteTimeout,
		IdleTimeout:       serverIdleTimeout,
	}
}

// extendDropDeadlines gives a bounded large browser upload enough time to
// stream to local storage. It replaces the ordinary 30-second connection
// deadlines but still prevents a stalled upload from holding a connection
// forever.
func extendDropDeadlines(w http.ResponseWriter) {
	controller := http.NewResponseController(w)
	deadline := time.Now().Add(dropTransferTimeout)
	_ = controller.SetReadDeadline(deadline)
	_ = controller.SetWriteDeadline(deadline)
}

// longOperationWriter clears the server's initial write deadline while a
// comparison computes. Once the handler starts its response, each successful
// write gets a fresh ordinary write window. Thus a legitimate multi-minute
// comparison or streaming download is not cut off merely because its total
// duration exceeds WriteTimeout, while a client that stops reading still times
// out.
type longOperationWriter struct {
	http.ResponseWriter
	controller *http.ResponseController
	started    bool
}

func longOperationResponseWriter(w http.ResponseWriter) http.ResponseWriter {
	controller := http.NewResponseController(w)
	_ = controller.SetWriteDeadline(time.Time{})
	return &longOperationWriter{ResponseWriter: w, controller: controller}
}

func (w *longOperationWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *longOperationWriter) WriteHeader(status int) {
	w.beginWrite()
	w.ResponseWriter.WriteHeader(status)
}

func (w *longOperationWriter) Write(p []byte) (int, error) {
	w.beginWrite()
	n, err := w.ResponseWriter.Write(p)
	if err == nil {
		_ = w.controller.SetWriteDeadline(time.Now().Add(serverWriteTimeout))
	}
	return n, err
}

func (w *longOperationWriter) beginWrite() {
	if w.started {
		return
	}
	w.started = true
	_ = w.controller.SetWriteDeadline(time.Now().Add(serverWriteTimeout))
}
