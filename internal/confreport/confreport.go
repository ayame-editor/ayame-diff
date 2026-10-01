// Package confreport builds the audit-oriented "confirmation report" a review
// session can be exported as (#296): what was compared, under which
// conditions, which differences were ignored and why, what stays unconfirmed,
// and enough state to reproduce the comparison.
//
// It is the confirmation sibling of internal/htmlreport (diff presentation)
// and internal/dirreport (folder summary): the same self-contained,
// standard-library-only, printable shape, but describing the review process
// rather than only the diff.
//
// The report deliberately never embeds compared content unless the caller opts
// in (BuildOptions.IncludeContent). Confirmation tracking arrives separately
// (#288); when it is absent the report says so in plain words instead of
// guessing.
package confreport

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/ayame-editor/ayame-diff/internal/linediff"
)

// Generator identifies the tool that produced a report.
const Generator = "ayame-diff"

// ReasonIgnored is recorded for every hunk the reviewer chose to ignore. The
// report keeps the count and the coordinates so a hidden decision stays
// auditable, mirroring the X-Ayame-Ignored-Hunks guarantee.
const ReasonIgnored = "manually ignored during review"

// ConfirmationUnavailableNote is the honest fallback when the build has no
// confirmed-hunk state to report (issue #288 is not merged here).
const ConfirmationUnavailableNote = "This ayame-diff build does not track confirmed hunks " +
	"(issue #288 is not part of this build). The ignored and reviewed lists below are " +
	"the only review state available; treat every retained difference as unconfirmed."

// NoContentNote explains the default for an exported report.
const NoContentNote = "Compared content is not embedded by default. Re-export with " +
	"content embedding enabled if the report must carry the lines."

// Input describes one compared side. Path metadata is gathered by the caller
// (the server) so this package stays free of filesystem access.
type Input struct {
	Label    string `json:"label"`
	Path     string `json:"path,omitempty"`
	AbsPath  string `json:"abs_path,omitempty"`
	Absent   bool   `json:"absent,omitempty"`
	Inline   bool   `json:"inline,omitempty"`
	Exists   bool   `json:"exists"`
	Size     int64  `json:"size"`
	ModTime  string `json:"mtime,omitempty"`
	Encoding string `json:"encoding,omitempty"`
	SHA256   string `json:"sha256,omitempty"`
}

// SyncPoint is a 0-based forced correspondence, kept JSON-compatible with the
// web UI's comparison state.
type SyncPoint struct {
	Old uint64 `json:"old"`
	New uint64 `json:"new"`
}

// Conditions is the complete set of options the comparison ran under.
type Conditions struct {
	Mode               string      `json:"mode"`
	Encoding           string      `json:"encoding,omitempty"`
	Window             uint64      `json:"window"`
	MaxHunks           int         `json:"max_hunks"`
	MaxLines           uint64      `json:"max_lines"`
	IgnoreCase         bool        `json:"ignore_case"`
	Whitespace         string      `json:"whitespace"`
	IgnoreEOL          bool        `json:"ignore_eol"`
	IgnoreTrailingEOL  bool        `json:"ignore_trailing_eol"`
	LineFilters        []string    `json:"line_filters"`
	SyncPoints         []SyncPoint `json:"sync_points"`
	DetectMoves        bool        `json:"detect_moves"`
	MoveMinLines       uint64      `json:"move_min_lines,omitempty"`
	Numeric            bool        `json:"numeric,omitempty"`
	Reverse            bool        `json:"reverse,omitempty"`
	IgnoredHunkIndexes []int       `json:"ignored_hunk_indexes"`
}

// Totals is the difference breakdown. Counts describe the hunks that remain
// after the ignored ones are removed.
type Totals struct {
	OldLines             uint64 `json:"old_lines"`
	NewLines             uint64 `json:"new_lines"`
	Hunks                int    `json:"hunks"`
	HunkCount            uint64 `json:"hunk_count"`
	OmittedHunks         uint64 `json:"omitted_hunks"`
	Added                uint64 `json:"added"`
	Deleted              uint64 `json:"deleted"`
	Modified             uint64 `json:"modified"`
	MovedBlocks          uint64 `json:"moved_blocks"`
	MovedLines           uint64 `json:"moved_lines"`
	MoveDetectionSkipped bool   `json:"move_detection_skipped"`
	IgnoredHunks         uint64 `json:"ignored_hunks"`
	WhitespaceOnly       int    `json:"whitespace_only_hunks"`
}

// IgnoredHunk records one difference the reviewer set aside, so the decision is
// visible even though the hunk is omitted from totals and the patch.
type IgnoredHunk struct {
	Index    int    `json:"index"`
	Kind     string `json:"kind"`
	OldStart uint64 `json:"old_start"`
	OldLen   uint64 `json:"old_len"`
	NewStart uint64 `json:"new_start"`
	NewLen   uint64 `json:"new_len"`
	MoveID   uint64 `json:"move_id,omitempty"`
	Reason   string `json:"reason"`
}

// Confirmation reports confirmed vs remaining hunks. Available is false when
// the running build has no confirmation state, in which case Note says so.
type Confirmation struct {
	Available        bool   `json:"available"`
	Confirmed        []int  `json:"confirmed_hunks,omitempty"`
	ConfirmedCount   int    `json:"confirmed_count"`
	Unconfirmed      []int  `json:"unconfirmed_hunks,omitempty"`
	UnconfirmedCount int    `json:"unconfirmed_count"`
	Note             string `json:"note,omitempty"`
}

// Review reports the older "read" tracking the UI already has: hunks the
// reviewer has scrolled through. It is deliberately not called "confirmed".
type Review struct {
	Available   bool  `json:"available"`
	Read        []int `json:"read_hunks,omitempty"`
	ReadCount   int   `json:"read_count"`
	Unread      []int `json:"unread_hunks,omitempty"`
	UnreadCount int   `json:"unread_count"`
}

// Reproduce carries the versioned comparison state (the same serialization the
// web UI puts in the URL fragment) so the report can be replayed.
type Reproduce struct {
	StateVersion int             `json:"state_version"`
	State        json.RawMessage `json:"state,omitempty"`
	URL          string          `json:"url,omitempty"`
	Fragment     string          `json:"hash_fragment,omitempty"`
	Note         string          `json:"note,omitempty"`
}

// ContentHunk is the optional, explicitly opted-in copy of a hunk's lines.
type ContentHunk struct {
	Index int      `json:"index"`
	Kind  string   `json:"kind"`
	Old   []string `json:"old,omitempty"`
	New   []string `json:"new,omitempty"`
}

// Content is the opt-in section; Included is false and Hunks empty by default.
type Content struct {
	Included bool          `json:"included"`
	Hunks    []ContentHunk `json:"hunks,omitempty"`
}

// Report is the complete confirmation record.
type Report struct {
	Generator       string        `json:"generator"`
	Version         string        `json:"version"`
	GeneratedAt     string        `json:"generated_at"`
	Mode            string        `json:"mode"`
	Inputs          []Input       `json:"inputs"`
	Conditions      Conditions    `json:"conditions"`
	Totals          Totals        `json:"totals"`
	IgnoredHunks    []IgnoredHunk `json:"ignored_hunks"`
	Confirmation    Confirmation  `json:"confirmation"`
	Review          Review        `json:"review"`
	Reproduce       Reproduce     `json:"reproduce"`
	ContentIncluded bool          `json:"content_included"`
	Notes           []string      `json:"notes,omitempty"`
	Content         *Content      `json:"content,omitempty"`
}

// BuildOptions assembles a report. Old and New are the line sources the diff
// ran over; Result must be the diff before ignored hunks are removed, because
// the ignored/confirmed/read indices are positions in that list.
type BuildOptions struct {
	Version        string
	GeneratedAt    time.Time
	Mode           string
	Inputs         []Input
	Conditions     Conditions
	Old            linediff.Lines
	New            linediff.Lines
	Result         linediff.Result
	ConfirmedHunks *[]int // nil when this build has no confirmation tracking (#288)
	ReadHunks      []int
	Reproduce      Reproduce
	IncludeContent bool
}

// Build renders opts into a Report. It never includes hunk content unless
// opts.IncludeContent is true.
func Build(opts BuildOptions) *Report {
	generated := opts.GeneratedAt
	if generated.IsZero() {
		generated = time.Now().UTC()
	}
	pre := append([]linediff.Hunk(nil), opts.Result.Hunks...)

	ignoredSet := ignoredIndexSet(pre, opts.Conditions.IgnoredHunkIndexes)
	ignored := make([]IgnoredHunk, 0, len(ignoredSet))
	for _, index := range sortedInts(ignoredSet) {
		h := pre[index]
		ignored = append(ignored, IgnoredHunk{
			Index: index, Kind: h.Kind.String(),
			OldStart: h.OldStart, OldLen: h.OldLen,
			NewStart: h.NewStart, NewLen: h.NewLen,
			MoveID: h.MoveID, Reason: ReasonIgnored,
		})
	}

	remaining := opts.Result
	remaining.Hunks = append([]linediff.Hunk(nil), opts.Result.Hunks...)
	linediff.IgnoreHunks(&remaining, opts.Conditions.IgnoredHunkIndexes)

	report := &Report{
		Generator:   Generator,
		Version:     opts.Version,
		GeneratedAt: generated.UTC().Format(time.RFC3339),
		Mode:        opts.Mode,
		Inputs:      opts.Inputs,
		Conditions:  normalizeConditions(opts.Conditions),
		Totals: Totals{
			OldLines:             remaining.OldLines,
			NewLines:             remaining.NewLines,
			Hunks:                len(remaining.Hunks),
			HunkCount:            remaining.HunkCount,
			OmittedHunks:         remaining.OmittedHunks,
			Added:                remaining.Added,
			Deleted:              remaining.Deleted,
			Modified:             remaining.Modified,
			MovedBlocks:          remaining.MovedBlocks,
			MovedLines:           remaining.MovedLines,
			MoveDetectionSkipped: remaining.MoveDetectionSkipped,
			IgnoredHunks:         remaining.IgnoredHunks,
		},
		IgnoredHunks: ignored,
		Reproduce:    opts.Reproduce,
	}
	report.Totals.WhitespaceOnly = whitespaceOnlyCount(remaining.Hunks, opts.Old, opts.New)
	report.Confirmation = buildConfirmation(pre, ignoredSet, opts.ConfirmedHunks)
	report.Review = buildReview(pre, ignoredSet, opts.ReadHunks)

	if opts.IncludeContent {
		report.ContentIncluded = true
		retainedIndexes := make([]int, 0, len(pre))
		for i := range pre {
			if !ignoredSet[i] {
				retainedIndexes = append(retainedIndexes, i)
			}
		}
		report.Content = &Content{Included: true, Hunks: contentHunks(retainedIndexes, remaining.Hunks, opts)}
	} else {
		report.Notes = append(report.Notes, NoContentNote)
	}
	if remaining.OmittedHunks > 0 {
		report.Notes = append(report.Notes, fmt.Sprintf(
			"%d hunks were omitted by the max-hunks limit, so totals and the whitespace-only count describe only the retained hunks.",
			remaining.OmittedHunks))
	}
	if report.Reproduce.Note != "" {
		report.Notes = append(report.Notes, report.Reproduce.Note)
	}
	return report
}

func normalizeConditions(c Conditions) Conditions {
	if c.LineFilters == nil {
		c.LineFilters = []string{}
	}
	if c.SyncPoints == nil {
		c.SyncPoints = []SyncPoint{}
	}
	if c.IgnoredHunkIndexes == nil {
		c.IgnoredHunkIndexes = []int{}
	}
	return c
}

// ignoredIndexSet mirrors linediff.IgnoreHunks: a moved pair is ignored
// together, so the report lists both halves even when only one was clicked.
func ignoredIndexSet(hunks []linediff.Hunk, indexes []int) map[int]bool {
	set := make(map[int]bool)
	moveIDs := make(map[uint64]bool)
	for _, index := range indexes {
		if index >= 0 && index < len(hunks) {
			set[index] = true
			if id := hunks[index].MoveID; id != 0 {
				moveIDs[id] = true
			}
		}
	}
	for i, h := range hunks {
		if h.MoveID != 0 && moveIDs[h.MoveID] {
			set[i] = true
		}
	}
	return set
}

func buildConfirmation(hunks []linediff.Hunk, ignored map[int]bool, confirmed *[]int) Confirmation {
	// With no confirmation tracking every retained difference stays unconfirmed,
	// which is worth listing even though Available is false.
	confirmedSet := map[int]bool{}
	available := confirmed != nil
	if available {
		confirmedSet = validIndexSet(hunks, *confirmed)
	}
	unconfirmed := make([]int, 0)
	for i := range hunks {
		if confirmedSet[i] || ignored[i] {
			continue
		}
		unconfirmed = append(unconfirmed, i)
	}
	result := Confirmation{
		Available:        available,
		Confirmed:        sortedInts(confirmedSet),
		ConfirmedCount:   len(confirmedSet),
		Unconfirmed:      unconfirmed,
		UnconfirmedCount: len(unconfirmed),
	}
	if !available {
		result.Note = ConfirmationUnavailableNote
	}
	return result
}

func buildReview(hunks []linediff.Hunk, ignored map[int]bool, read []int) Review {
	readSet := validIndexSet(hunks, read)
	unread := make([]int, 0)
	for i := range hunks {
		if readSet[i] || ignored[i] {
			continue
		}
		unread = append(unread, i)
	}
	return Review{
		Available:   true,
		Read:        sortedInts(readSet),
		ReadCount:   len(readSet),
		Unread:      unread,
		UnreadCount: len(unread),
	}
}

func validIndexSet(hunks []linediff.Hunk, indexes []int) map[int]bool {
	set := make(map[int]bool, len(indexes))
	for _, index := range indexes {
		if index >= 0 && index < len(hunks) {
			set[index] = true
		}
	}
	return set
}

// whitespaceOnlyCount counts retained hunks whose sides become identical once
// all whitespace is removed — the differences a whitespace-ignoring comparison
// would not have reported.
func whitespaceOnlyCount(hunks []linediff.Hunk, old, new linediff.Lines) int {
	count := 0
	for _, h := range hunks {
		if h.Kind == linediff.Insert {
			if allWhitespace(linesAt(new, h.NewStart, h.NewLen)) {
				count++
			}
			continue
		}
		if h.Kind == linediff.Delete {
			if allWhitespace(linesAt(old, h.OldStart, h.OldLen)) {
				count++
			}
			continue
		}
		if equalWithoutSpace(linesAt(old, h.OldStart, h.OldLen), linesAt(new, h.NewStart, h.NewLen)) {
			count++
		}
	}
	return count
}

func contentHunks(indexes []int, hunks []linediff.Hunk, opts BuildOptions) []ContentHunk {
	out := make([]ContentHunk, 0, len(hunks))
	for i, h := range hunks {
		index := 0
		if i < len(indexes) {
			index = indexes[i]
		}
		oldLines := linesAt(opts.Old, h.OldStart, h.OldLen)
		newLines := linesAt(opts.New, h.NewStart, h.NewLen)
		out = append(out, ContentHunk{
			Index: index,
			Kind:  h.Kind.String(),
			Old:   oldLines,
			New:   newLines,
		})
	}
	return out
}

func linesAt(lines linediff.Lines, start, count uint64) []string {
	out := make([]string, 0, count)
	for i := start; i < start+count; i++ {
		s, _ := lines.Line(i)
		out = append(out, s)
	}
	return out
}

func allWhitespace(lines []string) bool {
	for _, line := range lines {
		if strings.TrimFunc(line, unicode.IsSpace) != "" {
			return false
		}
	}
	return true
}

func equalWithoutSpace(old, new []string) bool {
	if len(old) != len(new) {
		return false
	}
	oldNorm := make([]string, len(old))
	newNorm := make([]string, len(new))
	for i := range old {
		oldNorm[i] = removeSpace(old[i])
	}
	for i := range new {
		newNorm[i] = removeSpace(new[i])
	}
	sort.Strings(oldNorm)
	sort.Strings(newNorm)
	for i := range oldNorm {
		if oldNorm[i] != newNorm[i] {
			return false
		}
	}
	return true
}

func removeSpace(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if !unicode.IsSpace(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func sortedInts(set map[int]bool) []int {
	out := make([]int, 0, len(set))
	for value := range set {
		out = append(out, value)
	}
	sort.Ints(out)
	return out
}

// EncodeFragment turns a versioned comparison-state object into the
// `compare=<base64url>` fragment the web UI reads, matching
// internal/server/web/urlstate.js. It rejects state that is not a v1 object or
// that would exceed the UI's URL-length ceiling, so the report never carries a
// link the UI would refuse.
func EncodeFragment(state json.RawMessage) (string, error) {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(state, &object); err != nil {
		return "", fmt.Errorf("comparison state is not a JSON object")
	}
	if raw, ok := object["v"]; !ok || string(raw) != "1" {
		return "", fmt.Errorf("unsupported comparison state version")
	}
	for _, required := range []string{"mode", "paths", "controls"} {
		if _, ok := object[required]; !ok {
			return "", fmt.Errorf("comparison state is missing %q", required)
		}
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, state); err != nil {
		return "", fmt.Errorf("comparison state is not valid JSON")
	}
	encoded := base64.RawURLEncoding.EncodeToString(compact.Bytes())
	if len(encoded) > 32*1024 {
		return "", fmt.Errorf("comparison state is too large for a reliable URL")
	}
	return "compare=" + encoded, nil
}

// WriteJSON writes the report as indented JSON.
func (r *Report) WriteJSON(w io.Writer) error {
	bw := bufio.NewWriter(w)
	encoder := json.NewEncoder(bw)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(r); err != nil {
		return err
	}
	return bw.Flush()
}

// WriteMarkdown writes a printable Markdown report.
func (r *Report) WriteMarkdown(w io.Writer) error {
	bw := bufio.NewWriter(w)
	fmt.Fprintf(bw, "# ayame-diff confirmation report\n\n")
	fmt.Fprintf(bw, "- Generator: %s %s\n", Generator, r.Version)
	fmt.Fprintf(bw, "- Generated: %s\n", r.GeneratedAt)
	fmt.Fprintf(bw, "- Mode: %s\n", r.Mode)
	fmt.Fprintf(bw, "- Content embedded: %s\n\n", yesNo(r.ContentIncluded))

	fmt.Fprintf(bw, "## Inputs\n\n")
	fmt.Fprintf(bw, "| Side | Path | Exists | Size (bytes) | Modified | Encoding | SHA-256 |\n")
	fmt.Fprintf(bw, "|---|---|---|---|---|---|---|\n")
	for _, input := range r.Inputs {
		path := input.Path
		if input.Inline {
			path = "(inline text)"
		}
		if input.Absent {
			path += " (absent)"
		}
		fmt.Fprintf(bw, "| %s | %s | %s | %d | %s | %s | %s |\n",
			mdCell(input.Label), mdCell(path), yesNo(input.Exists), input.Size,
			mdCell(input.ModTime), mdCell(input.Encoding), mdCell(input.SHA256))
	}
	fmt.Fprintf(bw, "\n")

	fmt.Fprintf(bw, "## Comparison conditions\n\n")
	writeConditionsMarkdown(bw, r.Conditions)

	fmt.Fprintf(bw, "## Difference totals\n\n")
	fmt.Fprintf(bw, "- Compared lines: %d left / %d right\n", r.Totals.OldLines, r.Totals.NewLines)
	fmt.Fprintf(bw, "- Retained hunks: %d (of %d counted; %d omitted by the max-hunks limit)\n",
		r.Totals.Hunks, r.Totals.HunkCount, r.Totals.OmittedHunks)
	fmt.Fprintf(bw, "- Added: %d\n- Deleted: %d\n- Modified: %d\n",
		r.Totals.Added, r.Totals.Deleted, r.Totals.Modified)
	fmt.Fprintf(bw, "- Moved blocks/lines: %d / %d\n", r.Totals.MovedBlocks, r.Totals.MovedLines)
	fmt.Fprintf(bw, "- Whitespace-only hunks: %d\n", r.Totals.WhitespaceOnly)
	fmt.Fprintf(bw, "- Ignored hunks: %d\n\n", r.Totals.IgnoredHunks)

	fmt.Fprintf(bw, "## Ignored hunks (%d)\n\n", len(r.IgnoredHunks))
	if len(r.IgnoredHunks) == 0 {
		fmt.Fprintf(bw, "None.\n\n")
	} else {
		fmt.Fprintf(bw, "| # | Kind | Old | New | Reason |\n|---|---|---|---|---|\n")
		for _, h := range r.IgnoredHunks {
			fmt.Fprintf(bw, "| %d | %s | -%d,%d | +%d,%d | %s |\n",
				h.Index, mdCell(h.Kind), h.OldStart+1, h.OldLen, h.NewStart+1, h.NewLen, mdCell(h.Reason))
		}
		fmt.Fprintf(bw, "\n")
	}

	fmt.Fprintf(bw, "## Confirmation\n\n")
	if r.Confirmation.Available {
		fmt.Fprintf(bw, "- Confirmed: %d %s\n", r.Confirmation.ConfirmedCount, indexList(r.Confirmation.Confirmed))
	} else {
		fmt.Fprintf(bw, "%s\n", r.Confirmation.Note)
	}
	fmt.Fprintf(bw, "- Unconfirmed (excluding ignored): %d %s\n\n",
		r.Confirmation.UnconfirmedCount, indexList(r.Confirmation.Unconfirmed))

	fmt.Fprintf(bw, "## Reviewed (read) hunks\n\n")
	fmt.Fprintf(bw, "- Read: %d %s\n", r.Review.ReadCount, indexList(r.Review.Read))
	fmt.Fprintf(bw, "- Unread (excluding ignored): %d %s\n\n", r.Review.UnreadCount, indexList(r.Review.Unread))

	fmt.Fprintf(bw, "## Reproduce this comparison\n\n")
	fmt.Fprintf(bw, "- Comparison-state version: %d\n", r.Reproduce.StateVersion)
	if r.Reproduce.URL != "" {
		fmt.Fprintf(bw, "- URL: %s\n", r.Reproduce.URL)
	}
	if r.Reproduce.Fragment != "" {
		fmt.Fprintf(bw, "- Hash fragment: `#%s`\n", r.Reproduce.Fragment)
	}
	if r.Reproduce.State != nil {
		fmt.Fprintf(bw, "- State:\n\n```json\n%s\n```\n", string(r.Reproduce.State))
	}
	if r.Reproduce.Note != "" {
		fmt.Fprintf(bw, "\n> %s\n", r.Reproduce.Note)
	}
	fmt.Fprintf(bw, "\n")

	if len(r.Notes) > 0 {
		fmt.Fprintf(bw, "## Notes\n\n")
		for _, note := range r.Notes {
			fmt.Fprintf(bw, "- %s\n", note)
		}
		fmt.Fprintf(bw, "\n")
	}

	if r.Content != nil && len(r.Content.Hunks) > 0 {
		fmt.Fprintf(bw, "## Embedded content\n\n")
		for _, hunk := range r.Content.Hunks {
			fmt.Fprintf(bw, "### Hunk %d (%s)\n\n", hunk.Index, hunk.Kind)
			if len(hunk.Old) > 0 {
				fmt.Fprintf(bw, "Left:\n\n```text\n%s\n```\n\n", strings.Join(hunk.Old, "\n"))
			}
			if len(hunk.New) > 0 {
				fmt.Fprintf(bw, "Right:\n\n```text\n%s\n```\n\n", strings.Join(hunk.New, "\n"))
			}
		}
	}
	return bw.Flush()
}

func writeConditionsMarkdown(bw *bufio.Writer, c Conditions) {
	fmt.Fprintf(bw, "- Mode: %s\n", mdCell(c.Mode))
	fmt.Fprintf(bw, "- Encoding hint: %s\n", mdCell(orNone(c.Encoding)))
	fmt.Fprintf(bw, "- Resync window: %d\n", c.Window)
	fmt.Fprintf(bw, "- Max hunks / max lines per hunk: %d / %d\n", c.MaxHunks, c.MaxLines)
	fmt.Fprintf(bw, "- Ignore case: %s\n", yesNo(c.IgnoreCase))
	fmt.Fprintf(bw, "- Whitespace: %s\n", mdCell(c.Whitespace))
	fmt.Fprintf(bw, "- Ignore EOL / trailing EOL: %s / %s\n", yesNo(c.IgnoreEOL), yesNo(c.IgnoreTrailingEOL))
	fmt.Fprintf(bw, "- Line filters: %s\n", filtersText(c.LineFilters))
	fmt.Fprintf(bw, "- Sync points: %s\n", syncText(c.SyncPoints))
	fmt.Fprintf(bw, "- Move detection: %s (min lines %d)\n", yesNo(c.DetectMoves), c.MoveMinLines)
	if c.Numeric || c.Reverse {
		fmt.Fprintf(bw, "- Sort: numeric=%s reverse=%s\n", yesNo(c.Numeric), yesNo(c.Reverse))
	}
	fmt.Fprintf(bw, "- Ignored hunk indexes: %s\n\n", indexList(c.IgnoredHunkIndexes))
}

// WriteHTML writes a self-contained, printable HTML report (inline CSS, no
// external assets), with a print stylesheet.
func (r *Report) WriteHTML(w io.Writer) error {
	bw := bufio.NewWriter(w)
	esc := html.EscapeString
	fmt.Fprintf(bw, "<!doctype html>\n<html><head><meta charset=\"utf-8\">"+
		"<meta name=\"viewport\" content=\"width=device-width,initial-scale=1\">"+
		"<title>ayame-diff confirmation report</title><style>%s</style></head><body>\n", htmlStyle)
	fmt.Fprintf(bw, "<header><h1>ayame-diff confirmation report</h1>"+
		"<p class=\"meta\">version %s &middot; generated %s &middot; mode %s &middot; content embedded: %s</p></header>\n",
		esc(r.Version), esc(r.GeneratedAt), esc(r.Mode), yesNo(r.ContentIncluded))
	fmt.Fprintf(bw, "<main>\n")

	fmt.Fprintf(bw, "<section><h2>Inputs</h2><table><thead><tr><th>Side</th><th>Path</th>"+
		"<th>Exists</th><th>Size (bytes)</th><th>Modified</th><th>Encoding</th><th>SHA-256</th></tr></thead><tbody>\n")
	for _, input := range r.Inputs {
		path := input.Path
		if input.Inline {
			path = "(inline text)"
		}
		if input.Absent {
			path += " (absent)"
		}
		fmt.Fprintf(bw, "<tr><td>%s</td><td class=\"path\">%s</td><td>%s</td><td class=\"num\">%d</td>"+
			"<td>%s</td><td>%s</td><td class=\"hash\">%s</td></tr>\n",
			esc(input.Label), esc(path), yesNo(input.Exists), input.Size,
			esc(input.ModTime), esc(input.Encoding), esc(input.SHA256))
	}
	fmt.Fprintf(bw, "</tbody></table></section>\n")

	fmt.Fprintf(bw, "<section><h2>Comparison conditions</h2><dl>\n")
	writeConditionHTML(bw, r.Conditions)
	fmt.Fprintf(bw, "</dl></section>\n")

	fmt.Fprintf(bw, "<section><h2>Difference totals</h2><div class=\"summary\">")
	for _, item := range []struct {
		label string
		value string
	}{
		{"Retained hunks", fmt.Sprint(r.Totals.Hunks)},
		{"Added", fmt.Sprint(r.Totals.Added)},
		{"Deleted", fmt.Sprint(r.Totals.Deleted)},
		{"Modified", fmt.Sprint(r.Totals.Modified)},
		{"Moved blocks", fmt.Sprint(r.Totals.MovedBlocks)},
		{"Whitespace-only", fmt.Sprint(r.Totals.WhitespaceOnly)},
		{"Ignored", fmt.Sprint(r.Totals.IgnoredHunks)},
	} {
		fmt.Fprintf(bw, "<span><b>%s</b> %s</span>", esc(item.value), esc(item.label))
	}
	fmt.Fprintf(bw, "</div><p class=\"meta\">Compared %d left / %d right lines; %d of %d counted hunks retained; %d omitted by the max-hunks limit.</p></section>\n",
		r.Totals.OldLines, r.Totals.NewLines, r.Totals.Hunks, r.Totals.HunkCount, r.Totals.OmittedHunks)

	fmt.Fprintf(bw, "<section><h2>Ignored hunks (%d)</h2>\n", len(r.IgnoredHunks))
	if len(r.IgnoredHunks) == 0 {
		fmt.Fprintf(bw, "<p>None.</p>\n")
	} else {
		fmt.Fprintf(bw, "<table><thead><tr><th>#</th><th>Kind</th><th>Old</th><th>New</th><th>Reason</th></tr></thead><tbody>\n")
		for _, h := range r.IgnoredHunks {
			fmt.Fprintf(bw, "<tr><td class=\"num\">%d</td><td>%s</td><td class=\"num\">-%d,%d</td>"+
				"<td class=\"num\">+%d,%d</td><td>%s</td></tr>\n",
				h.Index, esc(h.Kind), h.OldStart+1, h.OldLen, h.NewStart+1, h.NewLen, esc(h.Reason))
		}
		fmt.Fprintf(bw, "</tbody></table>\n")
	}
	fmt.Fprintf(bw, "</section>\n")

	fmt.Fprintf(bw, "<section><h2>Confirmation</h2>\n")
	if r.Confirmation.Available {
		fmt.Fprintf(bw, "<p>Confirmed: <b>%d</b> %s</p>", r.Confirmation.ConfirmedCount, esc(indexList(r.Confirmation.Confirmed)))
	} else {
		fmt.Fprintf(bw, "<p class=\"warn\">%s</p>\n", esc(r.Confirmation.Note))
	}
	fmt.Fprintf(bw, "<p>Unconfirmed (excluding ignored): <b>%d</b> %s</p>\n",
		r.Confirmation.UnconfirmedCount, esc(indexList(r.Confirmation.Unconfirmed)))
	fmt.Fprintf(bw, "<p>Reviewed (read): <b>%d</b> %s</p>", r.Review.ReadCount, esc(indexList(r.Review.Read)))
	fmt.Fprintf(bw, "<p>Unread (excluding ignored): <b>%d</b> %s</p>\n",
		r.Review.UnreadCount, esc(indexList(r.Review.Unread)))
	fmt.Fprintf(bw, "</section>\n")

	fmt.Fprintf(bw, "<section><h2>Reproduce this comparison</h2><dl>\n")
	fmt.Fprintf(bw, "<dt>State version</dt><dd>%d</dd>\n", r.Reproduce.StateVersion)
	if r.Reproduce.URL != "" {
		fmt.Fprintf(bw, "<dt>URL</dt><dd class=\"path\">%s</dd>\n", esc(r.Reproduce.URL))
	}
	if r.Reproduce.Fragment != "" {
		fmt.Fprintf(bw, "<dt>Hash fragment</dt><dd><code>#%s</code></dd>\n", esc(r.Reproduce.Fragment))
	}
	if r.Reproduce.State != nil {
		fmt.Fprintf(bw, "<dt>State</dt><dd><pre>%s</pre></dd>\n", esc(prettyJSON(r.Reproduce.State)))
	}
	fmt.Fprintf(bw, "</dl>")
	if r.Reproduce.Note != "" {
		fmt.Fprintf(bw, "<p class=\"warn\">%s</p>", esc(r.Reproduce.Note))
	}
	fmt.Fprintf(bw, "</section>\n")

	if len(r.Notes) > 0 {
		fmt.Fprintf(bw, "<section><h2>Notes</h2><ul>\n")
		for _, note := range r.Notes {
			fmt.Fprintf(bw, "<li>%s</li>\n", esc(note))
		}
		fmt.Fprintf(bw, "</ul></section>\n")
	}

	if r.Content != nil && len(r.Content.Hunks) > 0 {
		fmt.Fprintf(bw, "<section><h2>Embedded content</h2>\n")
		for _, hunk := range r.Content.Hunks {
			fmt.Fprintf(bw, "<h3>Hunk %d (%s)</h3>\n", hunk.Index, esc(hunk.Kind))
			if len(hunk.Old) > 0 {
				fmt.Fprintf(bw, "<h4>Left</h4><pre>%s</pre>\n", esc(strings.Join(hunk.Old, "\n")))
			}
			if len(hunk.New) > 0 {
				fmt.Fprintf(bw, "<h4>Right</h4><pre>%s</pre>\n", esc(strings.Join(hunk.New, "\n")))
			}
		}
		fmt.Fprintf(bw, "</section>\n")
	}

	fmt.Fprintf(bw, "</main></body></html>\n")
	return bw.Flush()
}

func writeConditionHTML(bw *bufio.Writer, c Conditions) {
	for _, item := range []struct{ k, v string }{
		{"Mode", c.Mode},
		{"Encoding hint", orNone(c.Encoding)},
		{"Resync window", fmt.Sprint(c.Window)},
		{"Max hunks", fmt.Sprint(c.MaxHunks)},
		{"Max lines per hunk", fmt.Sprint(c.MaxLines)},
		{"Ignore case", yesNo(c.IgnoreCase)},
		{"Whitespace", c.Whitespace},
		{"Ignore EOL", yesNo(c.IgnoreEOL)},
		{"Ignore trailing EOL", yesNo(c.IgnoreTrailingEOL)},
		{"Line filters", filtersText(c.LineFilters)},
		{"Sync points", syncText(c.SyncPoints)},
		{"Move detection", yesNo(c.DetectMoves)},
		{"Move min lines", fmt.Sprint(c.MoveMinLines)},
		{"Ignored hunk indexes", indexList(c.IgnoredHunkIndexes)},
	} {
		fmt.Fprintf(bw, "<dt>%s</dt><dd>%s</dd>", html.EscapeString(item.k), html.EscapeString(item.v))
	}
}

const htmlStyle = `
:root{--bg:#fff;--fg:#1c2128;--muted:#6b7480;--border:#d7dce1;--panel:#f6f8fa;--warn:#8a6500}
@media(prefers-color-scheme:dark){:root{--bg:#0d1117;--fg:#e6edf3;--muted:#8b949e;
--border:#30363d;--panel:#161b22;--warn:#efc96d}}
*{box-sizing:border-box}body{margin:0;background:var(--bg);color:var(--fg);
font:14px/1.5 system-ui,-apple-system,"Segoe UI",Meiryo,sans-serif}
header{padding:.8rem 1rem;border-bottom:1px solid var(--border)}
h1{margin:0;font-size:1.2rem}.meta{color:var(--muted);margin:.3rem 0 0}
main{max-width:1100px;margin:0 auto;padding:1rem}
section{margin-bottom:1.4rem}h2{font-size:1rem;border-bottom:1px solid var(--border);padding-bottom:.25rem}
table{border-collapse:collapse;width:100%}th,td{border:1px solid var(--border);padding:.3rem .5rem;
text-align:left;vertical-align:top}th{background:var(--panel)}
.num{text-align:right;font-variant-numeric:tabular-nums}
.path,.hash{font-family:ui-monospace,Menlo,Consolas,monospace;word-break:break-all}
.hash{font-size:.8rem}
.summary{display:flex;gap:1rem;flex-wrap:wrap}.summary b{font-variant-numeric:tabular-nums}
dl{display:grid;grid-template-columns:minmax(10rem,auto) 1fr;gap:.15rem .8rem;margin:0}
dt{color:var(--muted)}dd{margin:0;word-break:break-word}
pre{background:var(--panel);border:1px solid var(--border);border-radius:6px;padding:.5rem;
overflow-x:auto;white-space:pre-wrap;word-break:break-word}
.warn{border-left:4px solid var(--warn);background:var(--panel);padding:.5rem .7rem;border-radius:4px}
@media print{body{background:#fff;color:#000}header,section{break-inside:avoid}
summary,table{break-inside:avoid}pre{white-space:pre-wrap}}
`

func yesNo(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}

func orNone(value string) string {
	if value == "" {
		return "(none)"
	}
	return value
}

func mdCell(value string) string {
	value = strings.ReplaceAll(value, "\r", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	return strings.ReplaceAll(value, "|", "\\|")
}

func filtersText(filters []string) string {
	if len(filters) == 0 {
		return "(none)"
	}
	quoted := make([]string, len(filters))
	for i, filter := range filters {
		quoted[i] = "/" + filter + "/"
	}
	return strings.Join(quoted, " ")
}

func syncText(points []SyncPoint) string {
	if len(points) == 0 {
		return "(none)"
	}
	parts := make([]string, len(points))
	for i, point := range points {
		parts[i] = fmt.Sprintf("%d:%d", point.Old+1, point.New+1)
	}
	return strings.Join(parts, ", ")
}

func indexList(indexes []int) string {
	if len(indexes) == 0 {
		return "(none)"
	}
	parts := make([]string, len(indexes))
	for i, index := range indexes {
		parts[i] = fmt.Sprint(index)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func prettyJSON(raw json.RawMessage) string {
	var builder bytes.Buffer
	if err := json.Indent(&builder, raw, "", "  "); err != nil {
		return string(raw)
	}
	return builder.String()
}
