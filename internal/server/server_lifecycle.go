package server

import (
	"net/http"
	"strings"
	"sync"
	"time"
)

type browserSessionRequest struct {
	Session string `json:"session"`
}

type browserLifecycle struct {
	server       *Server
	leaseTimeout time.Duration
	closeGrace   time.Duration

	mu            sync.Mutex
	sessions      map[string]time.Time
	emptyDeadline time.Time
	wake          chan struct{}
}

func newBrowserLifecycle(server *Server, leaseTimeout, closeGrace time.Duration) *browserLifecycle {
	if closeGrace <= 0 || closeGrace > leaseTimeout {
		closeGrace = min(5*time.Second, leaseTimeout)
	}
	lifecycle := &browserLifecycle{
		server:        server,
		leaseTimeout:  leaseTimeout,
		closeGrace:    closeGrace,
		sessions:      make(map[string]time.Time),
		emptyDeadline: time.Now().Add(leaseTimeout),
		wake:          make(chan struct{}, 1),
	}
	go lifecycle.run()
	return lifecycle
}

func (l *browserLifecycle) run() {
	timer := time.NewTimer(l.leaseTimeout)
	defer timer.Stop()
	for {
		delay, expired := l.nextDelay(time.Now())
		if expired {
			l.server.requestShutdown()
			return
		}
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		timer.Reset(delay)
		select {
		case <-timer.C:
		case <-l.wake:
		}
	}
}

func (l *browserLifecycle) nextDelay(now time.Time) (time.Duration, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()

	var next time.Time
	for session, heartbeat := range l.sessions {
		deadline := heartbeat.Add(l.leaseTimeout)
		if !deadline.After(now) {
			delete(l.sessions, session)
			continue
		}
		if next.IsZero() || deadline.Before(next) {
			next = deadline
		}
	}
	if len(l.sessions) == 0 {
		if l.emptyDeadline.IsZero() {
			l.emptyDeadline = now.Add(l.closeGrace)
		}
		next = l.emptyDeadline
	}
	if !next.After(now) {
		return 0, true
	}
	return next.Sub(now), false
}

func (l *browserLifecycle) heartbeat(session string) {
	l.mu.Lock()
	l.sessions[session] = time.Now()
	l.emptyDeadline = time.Time{}
	l.mu.Unlock()
	l.notify()
}

func (l *browserLifecycle) release(session string) {
	l.mu.Lock()
	delete(l.sessions, session)
	if len(l.sessions) == 0 {
		l.emptyDeadline = time.Now().Add(l.closeGrace)
	}
	l.mu.Unlock()
	l.notify()
}

func (l *browserLifecycle) notify() {
	select {
	case l.wake <- struct{}{}:
	default:
	}
}

func validBrowserSession(session string) bool {
	if session == "" || len(session) > 128 {
		return false
	}
	return strings.IndexFunc(session, func(r rune) bool {
		return !(r >= 'a' && r <= 'z') &&
			!(r >= 'A' && r <= 'Z') &&
			!(r >= '0' && r <= '9') &&
			r != '-' && r != '_' && r != '.'
	}) == -1
}

func (s *Server) handleBrowserHeartbeat(w http.ResponseWriter, r *http.Request) {
	request, ok := decodePostJSON[browserSessionRequest](w, r, "")
	if !ok {
		return
	}
	if !validBrowserSession(request.Session) {
		writeError(w, http.StatusBadRequest, "a safe browser session ID is required")
		return
	}
	if s.lifecycle != nil {
		s.lifecycle.heartbeat(request.Session)
	}
	writeJSON(w, http.StatusOK, map[string]bool{"leased": s.lifecycle != nil})
}

func (s *Server) handleBrowserRelease(w http.ResponseWriter, r *http.Request) {
	request, ok := decodePostJSON[browserSessionRequest](w, r, "")
	if !ok {
		return
	}
	if !validBrowserSession(request.Session) {
		writeError(w, http.StatusBadRequest, "a safe browser session ID is required")
		return
	}
	if s.lifecycle != nil {
		s.lifecycle.release(request.Session)
	}
	writeJSON(w, http.StatusOK, map[string]bool{"released": true})
}

func (s *Server) handleShutdown(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	if s.shutdown == nil {
		writeError(w, http.StatusServiceUnavailable, "server shutdown is not available")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"shutting_down": true})
	s.requestShutdown()
}

func (s *Server) requestShutdown() {
	if s.shutdown != nil {
		s.shutdownOnce.Do(s.shutdown)
	}
}
