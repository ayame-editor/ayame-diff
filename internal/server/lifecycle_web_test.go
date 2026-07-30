package server

import (
	"strings"
	"testing"
)

// TestBrowserLifecycleUIIsWired keeps the visible stop control and the
// authenticated per-tab lease connected to their server endpoints (#96).
func TestBrowserLifecycleUIIsWired(t *testing.T) {
	t.Parallel()
	index := readWebAsset(t, "index.html")
	app := readWebAsset(t, "app.js")

	for _, want := range []string{
		`id="stopServer"`,
		`data-i18n="stopServer"`,
		`data-i18n-aria-label="stopServer"`,
	} {
		if !strings.Contains(index, want) {
			t.Errorf("lifecycle UI is missing %q", want)
		}
	}
	for _, want := range []string{
		`apiFetch("/api/shutdown"`,
		`postBrowserLifecycle("/api/lifecycle/heartbeat")`,
		`postBrowserLifecycle("/api/lifecycle/release")`,
		"keepalive: true",
		`window.addEventListener("pagehide"`,
		`window.addEventListener("pageshow", startBrowserSession)`,
		"BROWSER_HEARTBEAT_INTERVAL_MS",
	} {
		if !strings.Contains(app, want) {
			t.Errorf("browser lifecycle wiring is missing %q", want)
		}
	}
}
