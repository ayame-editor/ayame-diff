// Package server hosts a small local web UI for ayame-diff: it serves an
// embedded single-page frontend and a JSON diff API over localhost. It is the
// GUI foundation (hjosugi/ayame-diff#10); the diff view lives in the embedded
// web assets (#11). Dependency-zero: net/http, embed, and the diff packages.
package server

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/url"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/ayame-editor/ayame-diff/internal/engine"
)

//go:embed web
var webFS embed.FS

// Server-side resource caps bound what an (unauthenticated, #108) client can
// demand, so a single request cannot exhaust the host (#170).
const (
	// serverMaxMemoryText caps the resident-memory budget a CSV request may ask
	// for; larger values are clamped down — the engine just spills more, so the
	// result is unchanged.
	serverMaxMemoryText = "8GiB"
	// serverMaxArchiveEntryBytes / serverMaxArchiveBytes cap the uncompressed
	// archive expansion a dir request may request, so a client cannot raise the
	// zip-bomb guard (#70) back toward unbounded.
	serverMaxArchiveEntryBytes int64 = 2 << 30 // 2 GiB
	serverMaxArchiveBytes      int64 = 8 << 30 // 8 GiB
)

// Browser drops are staged on disk before the path-based comparison engines
// open them. Bound both one request and the aggregate for one browser session
// so a bad or accidental drop cannot grow one cache tree without limit (#109).
// Vars let regression tests exercise the boundaries with tiny payloads.
var (
	maxDropUploadBytes  int64 = 2 << 30 // 2 GiB per file
	maxDropSessionBytes int64 = 8 << 30 // 8 GiB per browser session
)

// maxConcurrentComparisons bounds how many expensive comparisons run at once, so
// N parallel requests cannot multiply memory / temp-dir / goroutine use without
// limit (#170). A var so tests can shrink it before New.
var maxConcurrentComparisons = max(2, runtime.NumCPU())

// dropSessionTTL is how long an unowned (previous-run) drop directory may sit
// before opportunistic cleanup removes it. Live sessions are excluded from
// cleanup regardless of age (#168). A var so tests can shrink it.
var dropSessionTTL = 24 * time.Hour

type dropSession struct {
	root string

	// Uploads within one browser session are serialized so aggregate accounting
	// remains exact. Different sessions can still stream concurrently.
	mu   sync.Mutex
	used int64
}

// Server serves the UI and diff API.
type Server struct {
	version      string
	token        string
	allowedHosts map[string]bool
	mux          *http.ServeMux
	dropMu       sync.Mutex
	drops        map[string]*dropSession
	compareSem   chan struct{} // bounds concurrent expensive comparisons (#170)
	watchSem     chan struct{} // bounds authenticated long-poll file watches (#251)
	shutdown     func()
	shutdownOnce sync.Once
	lifecycle    *browserLifecycle
}

// LifecycleOptions connects authenticated browser lifecycle events to the
// command that owns the HTTP server.
type LifecycleOptions struct {
	// Shutdown requests graceful process shutdown. It must return quickly; the
	// command owns the http.Server and performs the actual drain.
	Shutdown func()
	// BrowserLeaseTimeout enables browser-session leases. A positive value is
	// used by `gui` so a crashed or closed browser cannot leave an orphan
	// process. Zero disables lease-driven shutdown, as `serve` expects.
	BrowserLeaseTimeout time.Duration
	// BrowserCloseGrace allows a reloaded page to acquire a new lease after the
	// old document releases its lease.
	BrowserCloseGrace time.Duration
}

// Options configures a Server.
type Options struct {
	// Version is reported by /api/health.
	Version string
	// AllowedHosts is the exact set of Host header values the server answers
	// to, normally the loopback names for the bound port. It is the defense
	// against DNS rebinding: a page on any website can make the browser resolve
	// its own hostname to 127.0.0.1 and reach this server, but the request
	// still carries that site's name in Host, not ours (#108).
	//
	// Empty accepts any Host. That is only appropriate for a deliberately
	// remote listener, whose reachable names the process cannot enumerate;
	// there the token is the defense.
	AllowedHosts []string
	Lifecycle    LifecycleOptions
}

// New returns a Server that accepts any Host. Prefer NewWithOptions, which can
// pin the Host header; this form suits tests and callers whose listener a
// browser cannot reach.
func New(version string) (*Server, error) {
	return NewWithOptions(Options{Version: version})
}

// NewWithOptions returns a Server with a freshly generated API token. Every
// /api route except the health probe requires that token in an X-Ayame-Token
// header, so a website the user happens to be visiting cannot drive the local
// GUI's read and write endpoints (#108).
func NewWithOptions(opts Options) (*Server, error) {
	sub, err := fs.Sub(webFS, "web")
	if err != nil {
		return nil, err
	}
	token, err := newAPIToken()
	if err != nil {
		return nil, err
	}
	allowed := make(map[string]bool, len(opts.AllowedHosts))
	for _, host := range opts.AllowedHosts {
		allowed[strings.ToLower(host)] = true
	}
	s := &Server{
		version: opts.Version, token: token, allowedHosts: allowed,
		mux: http.NewServeMux(), drops: make(map[string]*dropSession),
		compareSem: make(chan struct{}, maxConcurrentComparisons),
		watchSem:   make(chan struct{}, maxConcurrentWatchRequests),
		shutdown:   opts.Lifecycle.Shutdown,
	}
	s.mux.Handle("/", http.FileServer(http.FS(sub)))
	s.mux.HandleFunc("/api/health", s.handleHealth)
	// Compute-heavy comparison / merge / export handlers run under the
	// concurrency gate (#170); cheap metadata handlers (health, files,
	// path-info, drop, project, csv/inspect) do not.
	s.mux.HandleFunc("/api/diff", s.limited(s.handleDiff))
	s.mux.HandleFunc("/api/patch", s.limited(s.handlePatch))
	s.mux.HandleFunc("/api/merge/text", s.limited(s.handleTextMerge))
	s.mux.HandleFunc("/api/three-way/text", s.limited(s.handleThreeWayText))
	s.mux.HandleFunc("/api/merge/three-way/text", s.limited(s.handleThreeWayTextMerge))
	s.mux.HandleFunc("/api/csv/inspect", s.handleCSVInspect)
	s.mux.HandleFunc("/api/csv/diff", s.limited(s.handleCSVDiff))
	s.mux.HandleFunc("/api/csv/export", s.limited(s.handleCSVExport))
	s.mux.HandleFunc("/api/merge/csv", s.limited(s.handleCSVMerge))
	s.mux.HandleFunc("/api/three-way/csv", s.limited(s.handleThreeWayCSV))
	s.mux.HandleFunc("/api/merge/three-way/csv", s.limited(s.handleThreeWayCSVMerge))
	s.mux.HandleFunc("/api/dir/diff", s.limited(s.handleDirDiff))
	s.mux.HandleFunc("/api/files", s.handleFiles)
	s.mux.HandleFunc("/api/path-info", s.handlePathInfo)
	s.mux.HandleFunc("/api/watch", s.handleWatch)
	s.mux.HandleFunc("/api/drop", s.handleDrop)
	s.mux.HandleFunc("/api/project/save", s.handleProjectSave)
	s.mux.HandleFunc("/api/project/load", s.handleProjectLoad)
	s.mux.HandleFunc("/api/dir/preview", s.limited(s.handleDirPreview))
	s.mux.HandleFunc("/api/lifecycle/heartbeat", s.handleBrowserHeartbeat)
	s.mux.HandleFunc("/api/lifecycle/release", s.handleBrowserRelease)
	s.mux.HandleFunc("/api/shutdown", s.handleShutdown)
	if opts.Lifecycle.Shutdown != nil && opts.Lifecycle.BrowserLeaseTimeout > 0 {
		s.lifecycle = newBrowserLifecycle(s, opts.Lifecycle.BrowserLeaseTimeout, opts.Lifecycle.BrowserCloseGrace)
	}
	return s, nil
}

// limited caps concurrency on expensive comparison handlers, returning 429 when
// maxConcurrentComparisons are already running so parallel requests cannot
// multiply resource use without bound (#170).
func (s *Server) limited(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w = longOperationResponseWriter(w)
		select {
		case s.compareSem <- struct{}{}:
			defer func() { <-s.compareSem }()
			h(w, r)
		default:
			w.Header().Set("Retry-After", "1")
			writeError(w, http.StatusTooManyRequests, "too many concurrent comparisons; please retry shortly")
		}
	}
}

// contentSecurityPolicy locks the embedded single-page UI to its own origin:
// scripts and styles load only from us (runtime layout styles set via the
// CSSOM still work; inline <style>/style="" are covered by 'unsafe-inline'),
// images allow local data: URIs, and framing, objects, workers, foreign
// connections, and form posts are denied. Mirrors the sibling ayame-editor
// policy so the family stays consistent. (#146)
const contentSecurityPolicy = "default-src 'self'; base-uri 'none'; connect-src 'self'; font-src 'self'; form-action 'none'; frame-ancestors 'none'; img-src 'self' data:; object-src 'none'; script-src 'self'; style-src 'self' 'unsafe-inline'; worker-src 'none'"

// Handler returns the HTTP handler wrapped in the security middleware, so every
// route (the embedded UI and the JSON API) gets hardening headers and CSRF
// protection whether it is served by `serve` or `gui`. The recovery wrapper is
// outermost so a panic raised inside the security checks is caught too.
func (s *Server) Handler() http.Handler {
	return recovered(s.allowedHost(secure(s.authenticated(s.mux))))
}

// Token returns the API token this server requires. The command that starts the
// server puts it in the URL it opens or prints, which is how the browser comes
// to hold it.
func (s *Server) Token() string { return s.token }

// tokenHeader carries the API token. A header is deliberate: a page on another
// origin cannot set one without a CORS preflight, and this server answers no
// preflight, so the token requirement doubles as CSRF protection. A cookie
// would be attached automatically and would not.
const tokenHeader = "X-Ayame-Token"

// newAPIToken returns a fresh 256-bit token for one server run.
func newAPIToken() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate API token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

// openPath reports whether a path is reachable without the token: the embedded
// UI itself, which holds no user data and cannot send headers for its own
// sub-resources, and the health probe, which reports only the version and is
// what the launching command polls for readiness.
func openPath(path string) bool {
	return path == "/api/health" || !strings.HasPrefix(path, "/api/")
}

// authenticated rejects API requests without the token (#108). Before this,
// any website the user visited could POST to the local server and have it read
// or overwrite arbitrary files, and could GET /api/files to enumerate the disk.
func (s *Server) authenticated(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if openPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		// Constant time so a wrong token cannot be discovered byte by byte.
		if subtle.ConstantTimeCompare([]byte(r.Header.Get(tokenHeader)), []byte(s.token)) != 1 {
			writeError(w, http.StatusUnauthorized, "missing or invalid API token; open the URL printed by ayame-diff, which carries it")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// allowedHost rejects a request whose Host header is not one this server was
// told to answer to. This is what stops DNS rebinding: the attacking page keeps
// its own hostname in Host even once that name resolves to 127.0.0.1 (#108).
func (s *Server) allowedHost(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(s.allowedHosts) > 0 && !s.allowedHosts[strings.ToLower(r.Host)] {
			writeError(w, http.StatusForbidden, "unexpected Host header; reach this server at the address it printed")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// recovered turns a panic in a handler into a 500 with the same JSON error
// shape as every other failure, instead of net/http silently closing the
// connection and leaving the GUI waiting on a request that never answers
// (#137). The process keeps serving; the stack goes to the server log so the
// bug stays reportable.
//
// The panic value is deliberately not sent to the client: it can carry
// filesystem paths or input fragments, and the user can do nothing with it. The
// log line is the diagnosable copy.
func recovered(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			value := recover()
			if value == nil {
				return
			}
			// A panic after the response started cannot be rewritten as a 500;
			// aborting tells net/http not to pretend the reply was complete.
			if value == http.ErrAbortHandler {
				panic(value)
			}
			log.Printf("panic serving %s %s: %v\n%s", r.Method, r.URL.Path, value, debug.Stack())
			writeError(w, http.StatusInternalServerError, "internal error: the comparison could not be completed")
		}()
		next.ServeHTTP(w, r)
	})
}

// maxJSONBodyBytes caps every JSON request body so a huge or slow POST cannot
// exhaust memory (#147). It is deliberately generous — real diff/merge requests
// carry paths, options, and merge choices (kilobytes), and even pasted "scratch"
// text is well under this — while still bounding worst-case allocation. File
// contents travel via /api/drop, which streams to disk and is exempt below. A
// var (not const) so tests can shrink it without allocating the full ceiling.
var maxJSONBodyBytes int64 = 64 << 20 // 64 MiB

// secure adds response-hardening headers to every reply (#146) and rejects
// cross-origin state-changing requests (#145). Without an Origin check a page
// on any other website can, while the user has this local server running, POST
// to e.g. /api/csv/export or /api/merge/* and write attacker-chosen files: the
// request targets 127.0.0.1 with a valid Host, `text/plain` dodges the CORS
// preflight, and the response being unreadable cross-origin does not stop the
// write side effect. Only an Origin check closes that hole.
func secure(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", contentSecurityPolicy)
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		if !safeMethod(r.Method) && !sameOriginRequest(r) {
			writeError(w, http.StatusForbidden, "cross-origin request rejected")
			return
		}
		// Bound JSON request bodies so a crafted or slow POST can't exhaust
		// memory (#147). /api/drop streams uploads to disk in bounded memory and
		// carries file contents, so it keeps its own handling and is exempt.
		if !safeMethod(r.Method) && r.URL.Path != "/api/drop" {
			r.Body = http.MaxBytesReader(w, r.Body, maxJSONBodyBytes)
		}
		next.ServeHTTP(w, r)
	})
}

// safeMethod reports whether the method only reads state. Cross-origin reads
// are already blocked from being *read back* by the same-origin policy, so the
// CSRF gate only needs to guard the state-changing verbs.
func safeMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	default:
		return false
	}
}

// sameOriginRequest is the CSRF gate. A browser always attaches an Origin on a
// cross-site request, so an absent Origin marks a non-browser client (curl, the
// native GUI shell) that a malicious page cannot drive — allow it. When present,
// the Origin's host:port must equal the request's own Host (true same-origin,
// which also holds when the user bound `--addr` to a LAN address), with
// loopback origins accepted as a fallback for proxy/port quirks on a local tool.
func sameOriginRequest(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return false
	}
	if strings.EqualFold(u.Host, r.Host) {
		return true
	}
	host := u.Hostname()
	if strings.EqualFold(host, "localhost") {
		return true
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		return true
	}
	return false
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "version": s.version})
}

// requireMethod centralizes the API's method guard while preserving the
// endpoint-specific handler's early-return shape.
func requireMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method == method {
		return true
	}
	writeError(w, http.StatusMethodNotAllowed, "use "+method)
	return false
}

// decodePostJSON applies the common POST guard and JSON decoding contract.
// invalidMessage lets endpoints retain an established, domain-specific error;
// an empty message reports the decoder error for diagnostics.
func decodePostJSON[T any](w http.ResponseWriter, r *http.Request, invalidMessage string) (T, bool) {
	if !requireMethod(w, r, http.MethodPost) {
		var request T
		return request, false
	}
	return decodeJSON[T](w, r, invalidMessage)
}

// decodeJSON decodes a request after endpoint-specific guards such as
// concurrency admission have run.
func decodeJSON[T any](w http.ResponseWriter, r *http.Request, invalidMessage string) (T, bool) {
	var request T
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		if invalidMessage == "" {
			invalidMessage = "invalid JSON: " + err.Error()
		}
		writeError(w, http.StatusBadRequest, invalidMessage)
		return request, false
	}
	return request, true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func writeClassifiedError(w http.ResponseWriter, err error, fallback int) {
	writeError(w, statusForError(err, fallback), err.Error())
}

func statusForError(err error, fallback int) int {
	switch {
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return http.StatusRequestTimeout
	case errors.Is(err, fs.ErrNotExist):
		return http.StatusNotFound
	case errors.Is(err, fs.ErrPermission):
		return http.StatusForbidden
	case errors.Is(err, engine.ErrUnresolvedRows):
		return http.StatusBadRequest
	default:
		return fallback
	}
}
