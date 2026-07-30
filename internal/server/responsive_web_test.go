package server

import (
	"strings"
	"testing"
)

// TestResponsiveLayoutUsesStableWidthSteps covers #107. The original page had
// one 720px breakpoint, so controls moved unpredictably through the common
// laptop split-view range and the centred result wasted wide screens.
func TestResponsiveLayoutUsesStableWidthSteps(t *testing.T) {
	t.Parallel()
	style := readWebAsset(t, "style.css")

	mainRule := sectionBetween(t, style, "main {", "}")
	if !strings.Contains(mainRule, "width: 100%") || strings.Contains(mainRule, "max-width") {
		t.Errorf("the result does not use the full wide-screen width: %s", mainRule)
	}

	mediumStart := strings.Index(style, "@media (max-width: 960px) {")
	narrowStart := strings.Index(style, "@media (max-width: 720px) {")
	if mediumStart < 0 || narrowStart < 0 || mediumStart >= narrowStart {
		t.Fatal("responsive breakpoints are missing or out of order")
	}
	medium := style[mediumStart:narrowStart]
	for _, want := range []string{
		"main { padding: 0.75rem; }",
		".project-actions { grid-template-columns: minmax(0, 1fr) auto auto; }",
		".project-actions select { grid-column: 1 / -1; }",
		".pane-head-meta { display: none; }",
	} {
		if !strings.Contains(medium, want) {
			t.Errorf("960px layout is missing %q", want)
		}
	}

	narrow := style[narrowStart:]
	for _, want := range []string{
		".scratch-area { grid-template-columns: 1fr; }",
		".project-actions { grid-template-columns: 1fr 1fr; }",
		".three-grid { grid-template-columns: 1fr; overflow: visible; }",
		".pane-heads.three { grid-template-columns: 1fr; }",
		".statusbar-paths { display: none; }",
	} {
		if !strings.Contains(narrow, want) {
			t.Errorf("720px layout is missing %q", want)
		}
	}

	tableWrap := sectionBetween(t, style, ".csv-table-wrap {", "}")
	if !strings.Contains(tableWrap, "overflow-x: auto") {
		t.Error("wide CSV data has no bounded horizontal scroll at responsive widths")
	}
}
