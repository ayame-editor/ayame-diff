package server

import (
	"encoding/json"
	"net/http"
	"os/exec"
	"reflect"
	"strings"
	"testing"
)

// whitespaceScaleFields is the shape web/whitespace-scale.js emits: the legacy
// /api/diff request fields a scale level is equivalent to (#259).
type whitespaceScaleFields struct {
	Whitespace        string `json:"whitespace"`
	IgnoreEOL         bool   `json:"ignoreEOL"`
	IgnoreTrailingEOL bool   `json:"ignoreTrailingEOL"`
}

// whitespaceScaleMapping runs the pure scale module under node and returns its
// level -> legacy request fields mapping. Skipped when node is unavailable.
func whitespaceScaleMapping(t *testing.T) map[string]whitespaceScaleFields {
	t.Helper()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is unavailable")
	}
	script := `
const scale = require('./web/whitespace-scale.js');
const out = {};
for (const level of scale.LEVELS) out[level] = scale.requestFields(level);
process.stdout.write(JSON.stringify(out));
`
	cmd := exec.Command(node, "-e", script)
	cmd.Dir = "."
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("whitespace-scale mapping failed: %v\n%s", err, output)
	}
	var mapping map[string]whitespaceScaleFields
	if err := json.Unmarshal(output, &mapping); err != nil {
		t.Fatalf("decode whitespace-scale mapping: %v (output=%s)", err, output)
	}
	return mapping
}

// TestWhitespaceScaleMapsToLegacyRequestFields pins the single monotonic scale
// to the exact request fields the old individual ignoreEOL / ignoreTrailingEOL /
// whitespace controls sent, so replacing the three controls stays backward
// compatible at the /api/diff boundary (#259).
func TestWhitespaceScaleMapsToLegacyRequestFields(t *testing.T) {
	mapping := whitespaceScaleMapping(t)
	want := map[string]whitespaceScaleFields{
		"strict":     {Whitespace: "none", IgnoreEOL: false, IgnoreTrailingEOL: false},
		"eol":        {Whitespace: "none", IgnoreEOL: true, IgnoreTrailingEOL: false},
		"eol-change": {Whitespace: "change", IgnoreEOL: true, IgnoreTrailingEOL: false},
		"eol-all":    {Whitespace: "all", IgnoreEOL: true, IgnoreTrailingEOL: false},
	}
	if !reflect.DeepEqual(mapping, want) {
		t.Fatalf("scale mapping = %+v, want %+v", mapping, want)
	}
}

// TestWhitespaceScaleMatchesLegacyFlagBehaviour checks that every point on the
// old 2x2x3 cube that the single scale still expresses produces the identical
// /api/diff response as the previous individual flags. ignoreEOL makes
// ignoreTrailingEOL redundant, so both values of the trailing toggle are covered
// for every EOL=true point; the whitespace-only-without-EOL points are the ones
// the P4Merge-style axis intentionally folds away, so they are not equivalent.
func TestWhitespaceScaleMatchesLegacyFlagBehaviour(t *testing.T) {
	mapping := whitespaceScaleMapping(t)
	h := newTestServer(t)

	samples := []struct{ name, old, new string }{
		{"eol_crlf", "x\r\ny\r\n", "x\ny\n"},
		{"whitespace_amount", "  a   b  \n", "a b\n"},
		{"whitespace_all", "a b c\n", "abc\n"},
		{"trailing_newline", "a\nb", "a\nb\n"},
		{"combined", "x\r\n  y  \r\n", "x\n y\n"},
		{"identical", "same\n", "same\n"},
	}

	type legacyPoint struct {
		level string
		req   diffRequest
	}
	levelForWhitespace := map[string]string{"none": "eol", "change": "eol-change", "all": "eol-all"}
	points := []legacyPoint{{level: "strict", req: diffRequest{Whitespace: "none"}}}
	for _, whitespace := range []string{"none", "change", "all"} {
		for _, trailing := range []bool{false, true} {
			points = append(points, legacyPoint{
				level: levelForWhitespace[whitespace],
				req:   diffRequest{Whitespace: whitespace, IgnoreEOL: true, IgnoreTrailingEOL: trailing},
			})
		}
	}

	for _, sample := range samples {
		for _, point := range points {
			t.Run(sample.name+"/"+point.level, func(t *testing.T) {
				legacy := point.req
				legacy.Inline = true
				legacy.OldText, legacy.NewText = sample.old, sample.new

				fields := mapping[point.level]
				scaled := legacy
				scaled.Whitespace = fields.Whitespace
				scaled.IgnoreEOL = fields.IgnoreEOL
				scaled.IgnoreTrailingEOL = fields.IgnoreTrailingEOL

				_, want := postDiff(t, h, legacy)
				rec, got := postDiff(t, h, scaled)
				if rec.Code != http.StatusOK {
					t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
				}
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("scale level %q (fields %+v) differs from legacy flags %+v\n got=%+v\nwant=%+v",
						point.level, fields, point.req, got, want)
				}
			})
		}
	}
}

// TestWhitespaceScaleAssetsAreWired keeps the module load order and consumption
// honest: the page loads whitespace-scale.js before app.js, the old three
// controls are gone, and app.js reads the module rather than re-deriving the
// mapping (#259). Runs without node so it always executes in CI.
func TestWhitespaceScaleAssetsAreWired(t *testing.T) {
	t.Parallel()
	index := readWebAsset(t, "index.html")
	app := readWebAsset(t, "app.js")

	if !strings.Contains(index, `src="whitespace-scale.js"`) {
		t.Error(`index.html missing <script src="whitespace-scale.js">`)
	}
	if strings.Index(index, `src="whitespace-scale.js"`) > strings.Index(index, `src="app.js"`) {
		t.Error("whitespace-scale.js must load before app.js")
	}
	for _, old := range []string{`id="ignoreEOL"`, `id="ignoreTrailingEOL"`, `id="whitespace"`} {
		if strings.Contains(index, old) {
			t.Errorf("index.html still has the legacy control %s", old)
		}
	}
	if !strings.Contains(index, `id="whitespaceScale"`) {
		t.Error(`index.html missing the unified scale select`)
	}
	for _, want := range []string{
		"globalThis.AyameWhitespaceScale",
		"whitespaceRequestFields(",
		"whitespaceScaleMode(",
		"whitespaceScaleLabelKey(",
	} {
		if !strings.Contains(app, want) {
			t.Errorf("app.js is missing scale wiring %q", want)
		}
	}
}
