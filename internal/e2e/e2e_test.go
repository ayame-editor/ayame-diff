package e2e

import (
	"bufio"
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ayame-editor/ayame-diff/internal/dircompare"
	"github.com/ayame-editor/ayame-diff/internal/engine"
	"github.com/ayame-editor/ayame-diff/internal/linediff"
	"github.com/ayame-editor/ayame-diff/internal/linesrc"
)

// ciSize is the default shape of each case: small enough that the whole
// benchmark suite and the regression gate run in a few minutes on a shared
// runner. AYAME_BENCH_LARGE=1 swaps in the sizes named in #279.
const (
	ciCSVRows     = 200_000
	ciTextBytes   = 16 << 20
	ciFolderFiles = 2_000
	ciLineBytes   = 4 << 20
)

// --- scale ---------------------------------------------------------------

// benchLarge reports whether the opt-in large sizes are active. CI never sets
// it; it exists so a developer can run the sizes named in the issue once.
func benchLarge() bool { return os.Getenv("AYAME_BENCH_LARGE") == "1" }

// scaleInt resolves one case size: an explicit environment override wins, then
// the large mode, then the small CI default.
func scaleInt(tb testing.TB, env string, small, large int) int {
	tb.Helper()
	if raw := os.Getenv(env); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n <= 0 {
			tb.Fatalf("%s=%q is not a positive integer", env, raw)
		}
		return n
	}
	if benchLarge() {
		return large
	}
	return small
}

func fileBytes(tb testing.TB, paths ...string) int64 {
	tb.Helper()
	var total int64
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			tb.Fatal(err)
		}
		total += info.Size()
	}
	return total
}

func appendPadded(dst []byte, n int64, width int) []byte {
	var scratch [20]byte
	digits := strconv.AppendInt(scratch[:0], n, 10)
	for pad := width - len(digits); pad > 0; pad-- {
		dst = append(dst, '0')
	}
	return append(dst, digits...)
}

// --- CSV ----------------------------------------------------------------

const csvHeader = "id,region,name,value\n"

// writeCSV writes a header plus rows of "id,region,name,value". When
// changedEvery > 0, every Nth row's value column is one larger, which is the
// right-hand side's only difference.
func writeCSV(tb testing.TB, path string, rows, changedEvery int) {
	tb.Helper()
	f, err := os.Create(path)
	if err != nil {
		tb.Fatal(err)
	}
	defer f.Close()
	w := bufio.NewWriterSize(f, 1<<20)
	if _, err := w.WriteString(csvHeader); err != nil {
		tb.Fatal(err)
	}
	line := make([]byte, 0, 64)
	for i := 0; i < rows; i++ {
		region := "JP"
		if i%3 == 0 {
			region = "US"
		}
		value := int64(i)
		if changedEvery > 0 && i%changedEvery == 0 {
			value++
		}
		line = line[:0]
		line = strconv.AppendInt(line, int64(i), 10)
		line = append(line, ',')
		line = append(line, region...)
		line = append(line, ",name-"...)
		line = strconv.AppendInt(line, int64(i), 10)
		line = append(line, ',')
		line = strconv.AppendInt(line, value, 10)
		line = append(line, '\n')
		if _, err := w.Write(line); err != nil {
			tb.Fatal(err)
		}
	}
	if err := w.Flush(); err != nil {
		tb.Fatal(err)
	}
}

type csvCase struct {
	left, right, out   string
	rows, changedEvery int
}

func newCSVCase(tb testing.TB, rows, changedEvery int) *csvCase {
	tb.Helper()
	dir := tb.TempDir()
	c := &csvCase{
		left:         filepath.Join(dir, "left.csv"),
		right:        filepath.Join(dir, "right.csv"),
		out:          filepath.Join(dir, "diff.tsv"),
		rows:         rows,
		changedEvery: changedEvery,
	}
	writeCSV(tb, c.left, rows, 0)
	writeCSV(tb, c.right, rows, changedEvery)
	return c
}

func csvRunConfig(left, right, out string) engine.Config {
	// Mirror the CLI defaults (workers and parse-workers are capped at 8) so
	// the numbers describe the shipped tool rather than a fixed small shape.
	workers := min(runtime.NumCPU(), 8)
	return engine.Config{
		LeftPath: left, RightPath: right, OutputPath: out,
		HasHeader: true, AlignColumnsByName: true,
		KeyNames:   []string{"id"},
		LeftFormat: "auto", RightFormat: "auto",
		LeftParser: "auto", RightParser: "auto",
		Partitions: 16, ParseWorkers: workers, Workers: workers,
		MemoryText: "256MiB", PartitionBufferText: "256KiB",
		MergeFanIn: 8, MaxRecordText: "8MiB",
		CellDiff: true, OutputHeader: true, Progress: false,
	}
}

func (c *csvCase) config() engine.Config {
	return csvRunConfig(c.left, c.right, c.out)
}

func (c *csvCase) run(tb testing.TB) (engine.Summary, time.Duration) {
	tb.Helper()
	start := time.Now()
	summary, err := engine.Run(context.Background(), c.config())
	elapsed := time.Since(start)
	if err != nil {
		tb.Fatalf("csv end-to-end run: %v", err)
	}
	return summary, elapsed
}

// changedRows counts the rows whose value column differs.
func (c *csvCase) changedRows() int {
	if c.changedEvery <= 0 {
		return 0
	}
	return (c.rows + c.changedEvery - 1) / c.changedEvery
}

// --- text ---------------------------------------------------------------

const (
	textPrefix = "line "
	textSuffix = " the quick brown fox jumps over the lazy dog 0123456789\n"
)

// writeText writes roughly targetBytes of fixed-width lines and returns how
// many lines it wrote. Every Nth line differs when changedEvery > 0.
func writeText(tb testing.TB, path string, targetBytes, changedEvery int) int {
	tb.Helper()
	lineLen := len(textPrefix) + 8 + len(textSuffix)
	lines := targetBytes / lineLen
	if lines < 1 {
		lines = 1
	}
	f, err := os.Create(path)
	if err != nil {
		tb.Fatal(err)
	}
	defer f.Close()
	w := bufio.NewWriterSize(f, 1<<20)
	line := make([]byte, 0, lineLen+8)
	for i := 0; i < lines; i++ {
		line = line[:0]
		line = append(line, textPrefix...)
		line = appendPadded(line, int64(i), 8)
		if changedEvery > 0 && i%changedEvery == 0 {
			line = append(line, " changed"...)
		}
		line = append(line, textSuffix...)
		if _, err := w.Write(line); err != nil {
			tb.Fatal(err)
		}
	}
	if err := w.Flush(); err != nil {
		tb.Fatal(err)
	}
	return lines
}

type textCase struct {
	left, right string
	lines       int
}

func newTextCase(tb testing.TB, targetBytes, changedEvery int) *textCase {
	tb.Helper()
	dir := tb.TempDir()
	c := &textCase{left: filepath.Join(dir, "left.txt"), right: filepath.Join(dir, "right.txt")}
	c.lines = writeText(tb, c.left, targetBytes, 0)
	writeText(tb, c.right, targetBytes, changedEvery)
	return c
}

// run compares the two files through the same streaming reader and bounded
// diff the CLI's text mode uses. maxLine is the per-line cap (0 disables it);
// maxHunks bounds retained hunks while the total is still counted.
func (c *textCase) run(tb testing.TB, maxLine, maxHunks int) (linediff.Result, time.Duration, error) {
	tb.Helper()
	oldFile, err := linesrc.OpenEncodingLimit(c.left, "auto", maxLine)
	if err != nil {
		return linediff.Result{}, 0, err
	}
	defer oldFile.Close()
	newFile, err := linesrc.OpenEncodingLimit(c.right, "auto", maxLine)
	if err != nil {
		return linediff.Result{}, 0, err
	}
	defer newFile.Close()
	start := time.Now()
	res, err := linediff.DiffWith(oldFile, newFile, linediff.Options{MaxHunks: maxHunks, Window: 64})
	elapsed := time.Since(start)
	if err != nil {
		return res, elapsed, err
	}
	// The reader's pre-count and the diff must agree, or a "completed" case
	// would really be a silent truncation.
	if res.OldLines != uint64(oldFile.Count()) || res.NewLines != uint64(newFile.Count()) {
		tb.Fatalf("line count mismatch: diff=%d/%d reader=%d/%d",
			res.OldLines, res.NewLines, oldFile.Count(), newFile.Count())
	}
	// Hunks are counted even when they are not retained, so the total must be
	// reconstructible from the bounded result.
	if res.HunkCount != uint64(len(res.Hunks))+res.OmittedHunks {
		tb.Fatalf("hunk accounting mismatch: count=%d len=%d omitted=%d",
			res.HunkCount, len(res.Hunks), res.OmittedHunks)
	}
	return res, elapsed, nil
}

// --- long line ----------------------------------------------------------

type longLineCase struct {
	left, right string
	size        int
}

func newLongLineCase(tb testing.TB, size int) *longLineCase {
	tb.Helper()
	dir := tb.TempDir()
	c := &longLineCase{
		left:  filepath.Join(dir, "long-left.txt"),
		right: filepath.Join(dir, "long-right.txt"),
		size:  size,
	}
	writeLongLine(tb, c.left, size, 'a')
	writeLongLine(tb, c.right, size, 'b')
	return c
}

// writeLongLine writes one line of size fill bytes and no terminator.
func writeLongLine(tb testing.TB, path string, size int, fill byte) {
	tb.Helper()
	data := make([]byte, size)
	for i := range data {
		data[i] = fill
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		tb.Fatal(err)
	}
}

func (c *longLineCase) run(tb testing.TB, maxLine int) (linediff.Result, time.Duration, error) {
	tb.Helper()
	oldFile, err := linesrc.OpenEncodingLimit(c.left, "auto", maxLine)
	if err != nil {
		return linediff.Result{}, 0, err
	}
	defer oldFile.Close()
	newFile, err := linesrc.OpenEncodingLimit(c.right, "auto", maxLine)
	if err != nil {
		return linediff.Result{}, 0, err
	}
	defer newFile.Close()
	start := time.Now()
	res, err := linediff.DiffWith(oldFile, newFile, linediff.Options{MaxHunks: 16, Window: 64})
	elapsed := time.Since(start)
	return res, elapsed, err
}

// --- folder -------------------------------------------------------------

func writeTree(tb testing.TB, root string, files, changedEvery int) {
	tb.Helper()
	for i := 0; i < files; i++ {
		rel := filepath.Join("group"+twoDigits(i%64), "file-"+sixDigits(i)+".txt")
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			tb.Fatal(err)
		}
		content := "row " + strconv.Itoa(i) + "\n"
		if changedEvery > 0 && i%changedEvery == 0 {
			content = "row " + strconv.Itoa(i) + " changed\n"
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			tb.Fatal(err)
		}
	}
}

func twoDigits(n int) string {
	if n < 10 {
		return "0" + strconv.Itoa(n)
	}
	return strconv.Itoa(n)
}

func sixDigits(n int) string {
	s := strconv.Itoa(n)
	for len(s) < 6 {
		s = "0" + s
	}
	return s
}

type folderCase struct {
	oldDir, newDir string
	files          int
}

func newFolderCase(tb testing.TB, files, changedEvery int) *folderCase {
	tb.Helper()
	dir := tb.TempDir()
	c := &folderCase{
		oldDir: filepath.Join(dir, "old"),
		newDir: filepath.Join(dir, "new"),
		files:  files,
	}
	writeTree(tb, c.oldDir, files, 0)
	writeTree(tb, c.newDir, files, changedEvery)
	return c
}

func (c *folderCase) run(tb testing.TB, maxEntries int) (*dircompare.Result, time.Duration, error) {
	tb.Helper()
	start := time.Now()
	res, err := dircompare.Compare(c.oldDir, c.newDir, dircompare.Options{
		Workers: runtime.NumCPU(), CompareBy: dircompare.CompareContents, MaxEntries: maxEntries,
	})
	return res, time.Since(start), err
}

// --- quick completeness and explicit-degradation checks -----------------

func TestEndToEndCSVCompletes(t *testing.T) {
	t.Parallel()
	const rows, changedEvery = 2000, 100
	c := newCSVCase(t, rows, changedEvery)
	summary, _ := c.run(t)
	changed := uint64(c.changedRows())
	if summary.LeftRows != rows || summary.RightRows != rows {
		t.Fatalf("rows left=%d right=%d want %d", summary.LeftRows, summary.RightRows, rows)
	}
	if summary.EqualRows != uint64(rows)-changed {
		t.Errorf("equal_rows=%d want %d", summary.EqualRows, uint64(rows)-changed)
	}
	if summary.DiffRows != 2*changed {
		t.Errorf("diff_rows=%d want %d", summary.DiffRows, 2*changed)
	}
	if summary.ChangedLeft != changed || summary.ChangedRight != changed {
		t.Errorf("changed left=%d right=%d want %d", summary.ChangedLeft, summary.ChangedRight, changed)
	}
}

func TestEndToEndTextCountsEveryHunk(t *testing.T) {
	t.Parallel()
	// Every line differs, so the total is far past the retained cap. The
	// omitted count must account for the difference instead of dropping hunks.
	const targetBytes, maxHunks = 64 << 10, 8
	c := newTextCase(t, targetBytes, 1)
	res, _, err := c.run(t, 0, maxHunks)
	if err != nil {
		t.Fatal(err)
	}
	if res.HunkCount <= uint64(len(res.Hunks)) {
		t.Fatalf("hunk_count=%d len=%d; expected the cap to omit some", res.HunkCount, len(res.Hunks))
	}
	if len(res.Hunks) != maxHunks {
		t.Errorf("retained hunks=%d want %d", len(res.Hunks), maxHunks)
	}
	if res.OmittedHunks == 0 {
		t.Error("omitted hunks were not reported")
	}
	if res.HunkCount != uint64(c.lines) {
		t.Errorf("hunk_count=%d want one per line (%d)", res.HunkCount, c.lines)
	}
}

func TestEndToEndFolderCompletes(t *testing.T) {
	t.Parallel()
	c := newFolderCase(t, 200, 10)
	res, _, err := c.run(t, 0)
	if err != nil {
		t.Fatal(err)
	}
	if res.Changed != 20 || res.Same != 180 || res.Added != 0 || res.Removed != 0 {
		t.Fatalf("changed=%d same=%d added=%d removed=%d want 20/180/0/0",
			res.Changed, res.Same, res.Added, res.Removed)
	}
	if len(res.Entries) != c.files {
		t.Fatalf("entries=%d want %d", len(res.Entries), c.files)
	}
}

func TestEndToEndLongLineCompletes(t *testing.T) {
	t.Parallel()
	c := newLongLineCase(t, 256<<10)
	res, _, err := c.run(t, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if res.HunkCount != 1 {
		t.Fatalf("hunk_count=%d want 1", res.HunkCount)
	}
}

func TestEndToEndLongLineDegradesExplicitly(t *testing.T) {
	t.Parallel()
	c := newLongLineCase(t, 256<<10)
	if _, _, err := c.run(t, 4<<10); err == nil {
		t.Fatal("an overlong line was accepted; the reader must refuse it at open time")
	} else {
		var tooLong *linesrc.LineTooLongError
		if !errors.As(err, &tooLong) {
			t.Fatalf("err=%T (%v) want *linesrc.LineTooLongError", err, err)
		}
		if tooLong.Limit != 4<<10 {
			t.Errorf("limit=%d want %d", tooLong.Limit, 4<<10)
		}
		for _, want := range []string{"--max-line-bytes", "binary (bin) mode"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("message %q does not mention %q", err.Error(), want)
			}
		}
	}
}

func TestEndToEndFolderEntryLimitDegradesExplicitly(t *testing.T) {
	t.Parallel()
	c := newFolderCase(t, 20, 1)
	_, _, err := c.run(t, 5)
	if !errors.Is(err, dircompare.ErrTooManyEntries) {
		t.Fatalf("err=%v want ErrTooManyEntries", err)
	}
	for _, want := range []string{"20", "5", "--max-entries"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("message %q does not mention %q", err.Error(), want)
		}
	}
}

func TestEndToEndCSVRecordLimitDegradesExplicitly(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	left, right := filepath.Join(dir, "left.csv"), filepath.Join(dir, "right.csv")
	out := filepath.Join(dir, "diff.tsv")
	huge := strings.Repeat("x", 2<<20) // 2 MiB in a single field
	content := "id,payload\n1," + huge + "\n"
	for _, path := range []string{left, right} {
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	cfg := csvRunConfig(left, right, out)
	cfg.MaxRecordText = "1MiB"
	_, err := engine.Run(context.Background(), cfg)
	if err == nil {
		t.Fatal("a record past --max-record-bytes was accepted")
	}
	if !strings.Contains(err.Error(), "larger than configured maximum") {
		t.Fatalf("error %q does not name the configured limit", err)
	}
}
