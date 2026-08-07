package threeway

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ayame-editor/ayame-diff/internal/atomicfile"
	"github.com/ayame-editor/ayame-diff/internal/encoding"
	"github.com/ayame-editor/ayame-diff/internal/engine"
)

type CSVEvent struct {
	ID    string     `json:"id"`
	Kind  Kind       `json:"kind"`
	Key   []string   `json:"key"`
	Base  [][]string `json:"base,omitempty"`
	Left  [][]string `json:"left,omitempty"`
	Right [][]string `json:"right,omitempty"`
	// Combined is set only for Merged groups: the result of applying both
	// sides' row edits, which is what the merge writes (#160).
	Combined [][]string `json:"combined,omitempty"`
}

type CSVResult struct {
	Header           []string   `json:"header"`
	HasHeader        bool       `json:"has_header"`
	KeyIndexes       []int      `json:"key_indexes"`
	Events           []CSVEvent `json:"events"`
	Conflicts        int        `json:"conflicts"`
	LeftOnly         int        `json:"left_only"`
	RightOnly        int        `json:"right_only"`
	Same             int        `json:"same_change"`
	Merged           int        `json:"merged"`
	BaseDelimiter    rune       `json:"-"`
	LazyQuotes       bool       `json:"-"`
	TrimLeadingSpace bool       `json:"-"`
	// BaseProfile carries the base file's byte-level conventions so the merge
	// reproduces them instead of normalizing to BOM-less UTF-8/LF (#159, #160).
	BaseProfile CSVProfile `json:"-"`
}

// CSVProfile captures the base file's character encoding, a leading UTF-8 BOM,
// and its line terminator, so WriteCSVMerge round-trips them rather than
// silently rewriting the merge as BOM-less UTF-8 with LF (#160).
type CSVProfile struct {
	Encoding string // concrete encoding name; "" or "utf-8" needs no re-encoding
	BOM      bool   // the base began with a UTF-8 BOM
	CRLF     bool   // rows are terminated with CRLF rather than LF
}

type csvDiffRecord struct {
	Kind string   `json:"kind"`
	Old  []string `json:"old"`
	New  []string `json:"new"`
}

// csvState accumulates one key group. Removals and additions are the row edits
// each side made; baseRows holds every base row carrying the key, including the
// ones no diff record mentions, so an untouched row in a duplicated key group
// is not invisible to the merge (#160).
type csvState struct {
	key                       []string
	leftRemoved, rightRemoved [][]string
	leftAdded, rightAdded     [][]string
	baseRows                  [][]string
	leftTouched, rightTouched bool
}

// CompareCSV runs the existing external-sort diff twice and joins only the
// changed key groups in memory. Inputs themselves remain streaming/bounded.
func CompareCSV(ctx context.Context, basePath, leftPath, rightPath string, cfg engine.Config) (CSVResult, error) {
	dir, err := os.MkdirTemp("", "ayame-three-way-csv-")
	if err != nil {
		return CSVResult{}, err
	}
	defer os.RemoveAll(dir)
	// The engine is byte-oriented and never decodes (internal/engine does not
	// import internal/encoding), yet it writes its diff as JSONL — and
	// encoding/json rewrites invalid UTF-8 as U+FFFD, so raw Shift_JIS bytes
	// are unrecoverable once they reach that output. Feed the engine UTF-8 and
	// keep the base's own conventions for the merge instead (#160).
	baseProfile, err := detectCSVProfile(basePath)
	if err != nil {
		return CSVResult{}, err
	}
	enginePaths := map[string]string{}
	for side, path := range map[string]string{"base": basePath, "left": leftPath, "right": rightPath} {
		profile := baseProfile
		if path != basePath {
			if profile, err = detectCSVProfile(path); err != nil {
				return CSVResult{}, err
			}
		}
		decoded, err := transcodeToUTF8(dir, side, path, profile)
		if err != nil {
			return CSVResult{}, err
		}
		enginePaths[side] = decoded
	}
	cfg.LeftPath, cfg.RightPath = enginePaths["base"], enginePaths["left"]
	inspection, err := engine.InspectInputs(cfg)
	if err != nil {
		return CSVResult{}, err
	}
	header := inspection.Header
	indexes, err := csvKeyIndexes(header, cfg)
	if err != nil {
		return CSVResult{}, err
	}
	states := make(map[string]*csvState)
	runSide := func(side string, path string) error {
		pair := cfg
		pair.LeftPath, pair.RightPath = enginePaths["base"], path
		pair.OutputPath = filepath.Join(dir, side+".jsonl")
		pair.CellDiff, pair.OutputFormat, pair.OutputHeader = true, "jsonl", false
		pair.Reconcile, pair.MergeChoices, pair.MergeDefault = false, nil, ""
		if _, err := engine.Run(ctx, pair); err != nil {
			return err
		}
		file, err := os.Open(pair.OutputPath)
		if err != nil {
			return err
		}
		defer file.Close()
		decoder := json.NewDecoder(file)
		for {
			var item csvDiffRecord
			if err := decoder.Decode(&item); err != nil {
				if errors.Is(err, io.EOF) {
					return nil
				}
				return err
			}
			removed, added := item.Old, item.New
			row := removed
			if len(row) == 0 {
				row = added
			}
			key, encoded, err := csvKey(row, indexes)
			if err != nil {
				return err
			}
			state := states[encoded]
			if state == nil {
				state = &csvState{key: key}
				states[encoded] = state
			}
			if side == "left" {
				state.leftTouched = true
				if len(removed) > 0 {
					state.leftRemoved = append(state.leftRemoved, removed)
				}
				if len(added) > 0 {
					state.leftAdded = append(state.leftAdded, added)
				}
			} else {
				state.rightTouched = true
				if len(removed) > 0 {
					state.rightRemoved = append(state.rightRemoved, removed)
				}
				if len(added) > 0 {
					state.rightAdded = append(state.rightAdded, added)
				}
			}
		}
	}
	if err := runSide("left", enginePaths["left"]); err != nil {
		return CSVResult{}, fmt.Errorf("base/left: %w", err)
	}
	if err := runSide("right", enginePaths["right"]); err != nil {
		return CSVResult{}, fmt.Errorf("base/right: %w", err)
	}
	result := CSVResult{Header: header, HasHeader: cfg.HasHeader, KeyIndexes: indexes, BaseDelimiter: inputDelimiter(basePath, cfg.LeftFormat, cfg.LeftDelimiter), LazyQuotes: cfg.LazyQuotes, TrimLeadingSpace: cfg.TrimLeadingSpace, BaseProfile: baseProfile}
	// Diff records only mention rows that changed, so re-read the base to pick
	// up the untouched rows of every changed key group. Memory stays bounded by
	// the changed groups, not by file size.
	if err := collectBaseGroups(basePath, baseProfile, cfg, result, states); err != nil {
		return CSVResult{}, err
	}
	keys := make([]string, 0, len(states))
	for key := range states {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, encoded := range keys {
		state := states[encoded]
		baseRows := state.baseRows
		leftRows := applyRowEdit(baseRows, state.leftRemoved, state.leftAdded)
		rightRows := applyRowEdit(baseRows, state.rightRemoved, state.rightAdded)
		combined, independent := combineRowEdits(baseRows, state)
		kind := Conflict
		switch {
		case !state.leftTouched:
			kind = RightOnly
		case !state.rightTouched:
			kind = LeftOnly
		case sameRowMultiset(leftRows, rightRows):
			kind = Same
		case sameRowMultiset(leftRows, baseRows):
			kind = RightOnly
		case sameRowMultiset(rightRows, baseRows):
			kind = LeftOnly
		case independent:
			kind = Merged
		}
		hash := sha256.Sum256([]byte(encoded))
		// Present every side in the base file's row positions, the same layout
		// the merge writes, so the GUI panes line up with the result (#160).
		event := CSVEvent{ID: hex.EncodeToString(hash[:16]), Kind: kind, Key: state.key, Base: baseRows, Left: orderLikeBase(baseRows, leftRows), Right: orderLikeBase(baseRows, rightRows)}
		if kind == Merged {
			event.Combined = orderLikeBase(baseRows, combined)
		}
		result.Events = append(result.Events, event)
		switch kind {
		case Conflict:
			result.Conflicts++
		case LeftOnly:
			result.LeftOnly++
		case RightOnly:
			result.RightOnly++
		case Same:
			result.Same++
		case Merged:
			result.Merged++
		}
	}
	return result, nil
}

// transcodeToUTF8 rewrites path under dir as BOM-less UTF-8 and returns the
// path the engine should read. A file that is already BOM-less UTF-8 is passed
// through untouched, so the common case costs nothing; gzip inputs stay
// compressed unless they also need decoding, since the engine expands .gz
// itself. Transcoding here is what lets a Shift_JIS, EUC-JP, UTF-16, or
// ISO-2022-JP three-way CSV work at all: the engine splits records on raw
// bytes and reports them through JSONL, neither of which survives a non-UTF-8
// payload (#160).
func transcodeToUTF8(dir, name, path string, profile CSVProfile) (string, error) {
	if isUTF8(profile.Encoding) && !profile.BOM {
		return path, nil
	}
	source, closeSource, err := openDecodedCSV(path, profile)
	if err != nil {
		return "", err
	}
	defer closeSource()
	// Keep the extension: the engine picks its delimiter and parser from it.
	target := filepath.Join(dir, name+filepath.Ext(strings.TrimSuffix(strings.ToLower(path), ".gz")))
	file, err := os.Create(target)
	if err != nil {
		return "", err
	}
	buffer := bufio.NewWriterSize(file, 256*1024)
	if _, err := io.Copy(buffer, source); err != nil {
		file.Close()
		return "", err
	}
	if err := buffer.Flush(); err != nil {
		file.Close()
		return "", err
	}
	return target, file.Close()
}

// collectBaseGroups streams the base file and records every row whose key names
// a changed group, in base order.
func collectBaseGroups(path string, profile CSVProfile, cfg engine.Config, result CSVResult, states map[string]*csvState) error {
	if len(states) == 0 {
		return nil
	}
	input, closeInput, err := openDecodedCSV(path, profile)
	if err != nil {
		return err
	}
	defer closeInput()
	reader := csv.NewReader(input)
	reader.Comma = result.BaseDelimiter
	reader.FieldsPerRecord = -1
	reader.ReuseRecord = true
	reader.LazyQuotes = result.LazyQuotes
	reader.TrimLeadingSpace = result.TrimLeadingSpace
	if cfg.HasHeader {
		if _, err := reader.Read(); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
	}
	for {
		row, err := reader.Read()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		_, encoded, err := csvKey(row, result.KeyIndexes)
		if err != nil {
			return err
		}
		if state := states[encoded]; state != nil {
			// ReuseRecord hands back the same backing array each call.
			state.baseRows = append(state.baseRows, append([]string(nil), row...))
		}
	}
}

// applyRowEdit returns base with one instance of each removed row taken out and
// the added rows appended, preserving base order for the rows that survive.
func applyRowEdit(base, removed, added [][]string) [][]string {
	pending := make(map[string]int, len(removed))
	for _, row := range removed {
		pending[rowString(row)]++
	}
	out := make([][]string, 0, len(base)+len(added))
	for _, row := range base {
		key := rowString(row)
		if pending[key] > 0 {
			pending[key]--
			continue
		}
		out = append(out, row)
	}
	return append(out, added...)
}

// combineRowEdits applies both sides' row edits to one key group. It reports
// false when the two sides consume the same base row instance — that is a real
// conflict the user must resolve. When they touch different rows the edits are
// independent and both apply, which is the duplicated-key case that used to be
// reported as a conflict whose every resolution dropped one side's edit (#160).
func combineRowEdits(base [][]string, state *csvState) ([][]string, bool) {
	if !state.leftTouched || !state.rightTouched {
		return nil, false
	}
	available := make(map[string]int, len(base))
	for _, row := range base {
		available[rowString(row)]++
	}
	removed := make([][]string, 0, len(state.leftRemoved)+len(state.rightRemoved))
	removed = append(removed, state.leftRemoved...)
	removed = append(removed, state.rightRemoved...)
	for _, row := range removed {
		key := rowString(row)
		if available[key] == 0 {
			return nil, false
		}
		available[key]--
	}
	combined := applyRowEdit(base, removed, nil)
	combined = append(combined, state.leftAdded...)
	return append(combined, state.rightAdded...), true
}

func csvKeyIndexes(header []string, cfg engine.Config) ([]int, error) {
	var indexes []int
	byName := make(map[string]int, len(header))
	for i, name := range header {
		byName[name] = i
	}
	for _, name := range cfg.KeyNames {
		index, ok := byName[name]
		if !ok {
			return nil, fmt.Errorf("key column %q not found", name)
		}
		indexes = append(indexes, index)
	}
	for _, index := range cfg.KeyIndexes {
		indexes = append(indexes, index-cfg.IndexBase)
	}
	if len(indexes) == 0 && len(cfg.ExcludeKeyNames)+len(cfg.ExcludeKeyIndexes) > 0 {
		excluded := make(map[int]bool)
		for _, name := range cfg.ExcludeKeyNames {
			index, ok := byName[name]
			if !ok {
				return nil, fmt.Errorf("excluded key column %q not found", name)
			}
			excluded[index] = true
		}
		for _, index := range cfg.ExcludeKeyIndexes {
			excluded[index-cfg.IndexBase] = true
		}
		for index := range header {
			if !excluded[index] {
				indexes = append(indexes, index)
			}
		}
	}
	if len(indexes) == 0 {
		return nil, fmt.Errorf("three-way CSV requires explicit key or exclude-key columns")
	}
	for _, index := range indexes {
		if index < 0 || index >= len(header) {
			return nil, fmt.Errorf("key index %d is out of range", index+cfg.IndexBase)
		}
	}
	return indexes, nil
}

func csvKey(row []string, indexes []int) ([]string, string, error) {
	key := make([]string, len(indexes))
	for i, index := range indexes {
		if index >= len(row) {
			return nil, "", fmt.Errorf("row has too few columns")
		}
		key[i] = row[index]
	}
	data, _ := json.Marshal(key)
	return key, string(data), nil
}
func rowString(row []string) string { data, _ := json.Marshal(row); return string(data) }

// sameRowMultiset compares two row groups ignoring order, without reordering
// either argument: both are emitted to the GUI and to the merge in base order.
func sameRowMultiset(a, b [][]string) bool {
	if len(a) != len(b) {
		return false
	}
	counts := make(map[string]int, len(a))
	for _, row := range a {
		counts[rowString(row)]++
	}
	for _, row := range b {
		key := rowString(row)
		counts[key]--
		if counts[key] < 0 {
			return false
		}
	}
	return true
}

// csvPlan maps a changed key group onto the base file's row positions: each
// base row is kept, replaced in place, or dropped, and result rows with no base
// row to sit in follow the group's last base row. This keeps a replacement
// where the row it replaces was instead of hoisting the whole group to its
// first occurrence (#160).
type csvPlan struct {
	emit   [][][]string
	tail   [][]string
	cursor int
	done   bool
}

func newCSVPlan(base, selected [][]string) *csvPlan {
	remaining := make(map[string]int, len(selected))
	for _, row := range selected {
		remaining[rowString(row)]++
	}
	kept := make([]bool, len(base))
	matched := make(map[string]int, len(base))
	for i, row := range base {
		key := rowString(row)
		if remaining[key] > 0 {
			remaining[key]--
			kept[i] = true
			matched[key]++
		}
	}
	// The selected rows that no base row hosts, in result order.
	var extras [][]string
	used := make(map[string]int, len(selected))
	for _, row := range selected {
		key := rowString(row)
		if used[key] < matched[key] {
			used[key]++
			continue
		}
		extras = append(extras, row)
	}
	plan := &csvPlan{emit: make([][][]string, len(base))}
	next := 0
	for i, row := range base {
		switch {
		case kept[i]:
			plan.emit[i] = [][]string{row}
		case next < len(extras):
			plan.emit[i] = [][]string{extras[next]}
			next++
		}
	}
	plan.tail = extras[next:]
	return plan
}

// orderLikeBase arranges rows the way WriteCSVMerge will emit them: rows the
// base already holds keep their positions and a replacement takes the slot of
// the row it replaces, with anything left over at the end.
func orderLikeBase(base, rows [][]string) [][]string {
	plan := newCSVPlan(base, rows)
	out := make([][]string, 0, len(rows))
	for _, emit := range plan.emit {
		out = append(out, emit...)
	}
	return append(out, plan.tail...)
}

// WriteCSVMerge streams the base file, replacing only event key groups. CSV
// conflicts default to BASE only after an explicit allowUnresolved decision.
func WriteCSVMerge(basePath, output string, result CSVResult, choices map[string]string, allowUnresolved bool) (unresolved int, resultErr error) {
	plans := make(map[string]*csvPlan, len(result.Events))
	order := make([]string, 0, len(result.Events))
	for _, event := range result.Events {
		encoded, _ := json.Marshal(event.Key)
		key := string(encoded)
		rows := event.Left
		switch event.Kind {
		case RightOnly:
			rows = event.Right
		case Same:
			rows = event.Left
		case Merged:
			rows = event.Combined
		case Conflict:
			switch choices[event.ID] {
			case "left":
				rows = event.Left
			case "right":
				rows = event.Right
			case "base":
				rows = event.Base
			default:
				unresolved++
				if !allowUnresolved {
					return unresolved, fmt.Errorf("%d CSV conflicts are unresolved", unresolved)
				}
				rows = event.Base
			}
		}
		plans[key] = newCSVPlan(event.Base, rows)
		order = append(order, key)
	}
	sort.Strings(order)
	profile := result.BaseProfile
	if profile.Encoding == "" {
		// Defensive: a caller that built the result by hand still gets a merge
		// in the base file's encoding rather than a silent UTF-8 rewrite.
		if detected, err := detectCSVProfile(basePath); err == nil {
			profile = detected
		}
	}
	input, closeInput, err := openDecodedCSV(basePath, profile)
	if err != nil {
		return unresolved, err
	}
	defer closeInput()
	writeErr := atomicfile.Write(output, atomicfile.Options{Pattern: ".ayame-three-way-csv-*"}, func(temp io.Writer) (writeResultErr error) {
		var destination io.Writer = temp
		var gz *gzip.Writer
		gzOpen := false
		defer func() {
			if gzOpen {
				if err := gz.Close(); writeResultErr == nil && err != nil {
					writeResultErr = err
				}
			}
		}()
		if strings.HasSuffix(strings.ToLower(output), ".gz") {
			gz = gzip.NewWriter(temp)
			gzOpen = true
			destination = gz
		}
		// A UTF-8 BOM is written raw ahead of the encoder, mirroring WriteMerged.
		if profile.BOM && isUTF8(profile.Encoding) {
			if _, err := destination.Write([]byte{0xEF, 0xBB, 0xBF}); err != nil {
				return err
			}
		}
		encoded := encoding.Encoder(flushOnlyWriter{destination}, profile.Encoding)
		buffer := bufio.NewWriterSize(encoded, 256*1024)
		writer := csv.NewWriter(buffer)
		outputLower := strings.TrimSuffix(strings.ToLower(output), ".gz")
		writer.Comma = ','
		if strings.HasSuffix(outputLower, ".tsv") {
			writer.Comma = '\t'
		}
		writer.UseCRLF = profile.CRLF
		reader := csv.NewReader(input)
		reader.Comma = result.BaseDelimiter
		reader.FieldsPerRecord = -1
		reader.ReuseRecord = true
		reader.LazyQuotes = result.LazyQuotes
		reader.TrimLeadingSpace = result.TrimLeadingSpace
		if result.HasHeader {
			if _, err := reader.Read(); err != nil {
				return err
			}
			if err := writer.Write(result.Header); err != nil {
				return err
			}
		}
		writeRows := func(rows [][]string) error {
			for _, row := range rows {
				if err := writer.Write(row); err != nil {
					return err
				}
			}
			return nil
		}
		for {
			row, err := reader.Read()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return err
			}
			_, encodedKey, err := csvKey(row, result.KeyIndexes)
			if err != nil {
				return err
			}
			plan := plans[encodedKey]
			if plan == nil {
				if err := writer.Write(row); err != nil {
					return err
				}
				continue
			}
			if plan.cursor >= len(plan.emit) {
				// The base grew between compare and write; keep the extra row
				// rather than dropping data.
				if err := writer.Write(row); err != nil {
					return err
				}
				continue
			}
			if err := writeRows(plan.emit[plan.cursor]); err != nil {
				return err
			}
			plan.cursor++
			if plan.cursor == len(plan.emit) {
				if err := writeRows(plan.tail); err != nil {
					return err
				}
				plan.done = true
			}
		}
		// Groups whose key never appears in the base (rows added on a side) and
		// any group the stream did not reach are emitted last, in key order.
		for _, key := range order {
			plan := plans[key]
			if plan.done {
				continue
			}
			for ; plan.cursor < len(plan.emit); plan.cursor++ {
				if err := writeRows(plan.emit[plan.cursor]); err != nil {
					return err
				}
			}
			if err := writeRows(plan.tail); err != nil {
				return err
			}
			plan.done = true
		}
		writer.Flush()
		if err := writer.Error(); err != nil {
			return err
		}
		if err := buffer.Flush(); err != nil {
			return err
		}
		if closer, ok := encoded.(io.Closer); ok {
			if err := closer.Close(); err != nil {
				return err
			}
		}
		if gz != nil {
			if err := gz.Close(); err != nil {
				return err
			}
			gzOpen = false
		}
		return nil
	})
	return unresolved, writeErr
}

func inputDelimiter(path, format, explicit string) rune {
	if explicit != "" {
		for _, value := range explicit {
			return value
		}
	}
	if strings.EqualFold(format, "tsv") {
		return '\t'
	}
	lower := strings.TrimSuffix(strings.ToLower(path), ".gz")
	if strings.HasSuffix(lower, ".tsv") {
		return '\t'
	}
	return ','
}

// openRaw opens path, transparently decompressing a .gz, without decoding.
func openRaw(path string) (io.Reader, func(), error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, func() {}, err
	}
	var reader io.Reader = file
	var gz *gzip.Reader
	if strings.HasSuffix(strings.ToLower(path), ".gz") {
		gz, err = gzip.NewReader(file)
		if err != nil {
			file.Close()
			return nil, func() {}, err
		}
		reader = gz
	}
	return reader, func() {
		if gz != nil {
			_ = gz.Close()
		}
		_ = file.Close()
	}, nil
}

// detectCSVProfile samples the head of path to determine the conventions a
// merge of it must reproduce: character encoding, a leading UTF-8 BOM, and
// whether rows end with CRLF.
func detectCSVProfile(path string) (CSVProfile, error) {
	reader, closeFn, err := openRaw(path)
	if err != nil {
		return CSVProfile{}, err
	}
	sample, err := io.ReadAll(io.LimitReader(reader, 8192))
	closeFn()
	if err != nil {
		return CSVProfile{}, err
	}
	profile := CSVProfile{Encoding: encoding.Detect(sample, encoding.Auto), BOM: bytes.HasPrefix(sample, []byte{0xEF, 0xBB, 0xBF})}
	// The sample can end mid-character, so a decode failure only costs the EOL
	// hint and leaves the LF default.
	if decoded, err := io.ReadAll(encoding.Decoder(bytes.NewReader(sample), profile.Encoding)); err == nil {
		profile.CRLF = bytes.Contains(decoded, []byte("\r\n"))
	}
	return profile, nil
}

// openDecodedCSV opens path as UTF-8 text using profile's encoding, discarding
// a leading BOM so the first field matches the decoded engine rows.
func openDecodedCSV(path string, profile CSVProfile) (io.Reader, func(), error) {
	reader, closeFn, err := openRaw(path)
	if err != nil {
		return nil, func() {}, err
	}
	decoded := bufio.NewReader(encoding.Decoder(reader, profile.Encoding))
	if prefix, _ := decoded.Peek(3); bytes.Equal(prefix, []byte{0xEF, 0xBB, 0xBF}) {
		_, _ = decoded.Discard(3)
	}
	return decoded, closeFn, nil
}
