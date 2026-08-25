package server

import (
	"strings"
	"testing"
)

// TestMessageLaneAssetsAreWired guards the browser integration around the
// execution-tested message log. Progress and outcomes must stay in separate
// lanes so one operation's result cannot erase the previous one (#97).
func TestMessageLaneAssetsAreWired(t *testing.T) {
	t.Parallel()

	index := readWebAsset(t, "index.html")
	app := readWebAsset(t, "app.js")
	style := readWebAsset(t, "style.css")

	if !strings.Contains(index, `<script src="messages.js"></script>`) {
		t.Error("index.html does not load messages.js")
	}
	if strings.Index(index, `src="messages.js"`) > strings.Index(index, `src="app.js"`) {
		t.Error("messages.js must load before app.js")
	}
	if !strings.Contains(index, `<div class="messages" id="messages"`) {
		t.Error("index.html is missing the message lane")
	}
	if !strings.Contains(index, `<section class="status" id="status"`) {
		t.Error("index.html lost the progress lane")
	}

	for _, want := range []string{
		"globalThis.AyameMessages",
		"createMessageLog({ onChange: renderMessages })",
		"function setProgress(",
		"function dismissMessage(",
		`t("dismissMessage")`,
		`t("messageRepeated"`,
	} {
		if !strings.Contains(app, want) {
			t.Errorf("app.js is missing message lane wiring %q", want)
		}
	}

	// Progress writes one lane, an outcome ends progress and posts to the other.
	body := renderFunctionBody(t, app, "function setStatus(")
	for _, want := range []string{
		`if (cls === "busy") { setProgress(msg); return; }`,
		`setProgress("");`,
		`messageLog.post(msg, cls || "info")`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("setStatus does not route %q", want)
		}
	}

	for _, want := range []string{".messages {", ".message.error", ".message.warning", ".message.success", ".message-dismiss"} {
		if !strings.Contains(style, want) {
			t.Errorf("style.css missing %q", want)
		}
	}
}

// TestOperationOutcomesUseTheirOwnTone keeps successes out of the neutral note
// tone: a saved merge or an exported patch must read as a success, and a
// failure must never be posted as one (#97).
func TestOperationOutcomesUseTheirOwnTone(t *testing.T) {
	t.Parallel()

	app := readWebAsset(t, "app.js")
	for _, want := range []string{
		`setStatus(t("mergeSaved", data.output), "success")`,
		`setStatus(t("projectSaved"), "success")`,
		`setStatus(t("exported"), "success")`,
		`setStatus(t("exportedCSV", data.output), "success")`,
		`setStatus(t("copied"), "success")`,
	} {
		if !strings.Contains(app, want) {
			t.Errorf("app.js is missing %q", want)
		}
	}
	for _, line := range strings.Split(app, "\n") {
		if strings.Contains(line, `err.message || err), "success")`) {
			t.Errorf("a failure is posted as a success: %s", line)
		}
	}
}
