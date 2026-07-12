// Package dirreport renders shareable folder-comparison reports.
package dirreport

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"html"
	"io"
	"strings"
	"time"

	"github.com/hjosugi/ayame-diff/internal/dircompare"
)

const style = `
:root{color-scheme:light dark;--bg:#fff;--fg:#242038;--muted:#706987;--panel:#faf8f3;
--border:#ded8cc;--add:#19733c;--remove:#b63856;--change:#8a6500;--same:#706987}
@media(prefers-color-scheme:dark){:root{--bg:#17151d;--fg:#f1edf8;--muted:#aaa2bc;
--panel:#211e29;--border:#3c3748;--add:#72d798;--remove:#ff8fa8;--change:#efc96d;--same:#aaa2bc}}
*{box-sizing:border-box}body{margin:0;background:var(--bg);color:var(--fg);
font:14px/1.5 system-ui,-apple-system,"Segoe UI",Meiryo,sans-serif}
header{padding:1rem;border-bottom:1px solid var(--border)}h1{margin:0;font-size:1.2rem}
.summary{display:flex;gap:1rem;flex-wrap:wrap;padding:.75rem 1rem;color:var(--muted)}
.summary b{color:var(--fg)}main{max-width:1100px;margin:auto;padding:0 1rem 2rem}
.tree{border:1px solid var(--border);border-radius:8px;overflow:hidden}
.entry{display:grid;grid-template-columns:minmax(15rem,1fr) 7rem 7rem 12rem 12rem;
gap:.75rem;padding:.4rem .65rem;border-top:1px solid var(--border);align-items:center}
.entry:first-child{border-top:0}.entry:nth-child(even){background:var(--panel)}
.path{font-family:ui-monospace,Menlo,Consolas,monospace;padding-left:calc(var(--depth)*1.1rem)}
.meta{color:var(--muted);font-variant-numeric:tabular-nums;font-size:.85rem}
.added .path{color:var(--add)}.removed .path{color:var(--remove)}
.changed .path{color:var(--change)}.same .path{color:var(--same)}
@media(max-width:800px){.entry{grid-template-columns:1fr auto}.mtime{display:none}}
`

// WriteHTML writes a self-contained folder tree report. Same entries are
// included only when all is true.
func WriteHTML(w io.Writer, res *dircompare.Result, title string, all bool) error {
	bw := bufio.NewWriter(w)
	esc := html.EscapeString
	fmt.Fprintf(bw, "<!doctype html>\n<html><head><meta charset=\"utf-8\">"+
		"<meta name=\"viewport\" content=\"width=device-width,initial-scale=1\">"+
		"<title>ayame-diff folder report: %s</title><style>%s</style></head><body>\n", esc(title), style)
	fmt.Fprintf(bw, "<header><h1>ayame-diff folder report</h1><div>%s</div></header>\n", esc(title))
	fmt.Fprintf(bw, "<div class=\"summary\"><span><b>%d</b> added</span><span><b>%d</b> removed</span>"+
		"<span><b>%d</b> changed</span><span><b>%d</b> same</span></div>\n<main><div class=\"tree\">\n",
		res.Added, res.Removed, res.Changed, res.Same)
	written := 0
	for _, entry := range res.Entries {
		if entry.Status == dircompare.Same && !all {
			continue
		}
		depth := strings.Count(entry.Path, "/")
		fmt.Fprintf(bw, "<div class=\"entry %s\"><span class=\"path\" style=\"--depth:%d\">%s %s</span>"+
			"<span class=\"meta\">%s</span><span class=\"meta\">%s</span>"+
			"<span class=\"meta mtime\">%s</span><span class=\"meta mtime\">%s</span></div>\n",
			entry.Status.String(), depth, statusMarker(entry.Status), esc(entry.Path), formatSize(entry.OldSize, entry.Status == dircompare.Added),
			formatSize(entry.NewSize, entry.Status == dircompare.Removed), formatTime(entry.OldModTime), formatTime(entry.NewModTime))
		written++
	}
	if written == 0 {
		fmt.Fprintln(bw, `<div class="entry same"><span class="path">No differences.</span></div>`)
	}
	fmt.Fprintln(bw, "</div></main></body></html>")
	return bw.Flush()
}

// WriteCSV writes a machine-readable RFC 4180 summary. Same entries are
// included only when all is true.
func WriteCSV(w io.Writer, res *dircompare.Result, all bool) error {
	cw := csv.NewWriter(w)
	if err := cw.Write([]string{"status", "path", "old_size", "new_size", "old_mtime", "new_mtime"}); err != nil {
		return err
	}
	for _, entry := range res.Entries {
		if entry.Status == dircompare.Same && !all {
			continue
		}
		if err := cw.Write([]string{
			entry.Status.String(), entry.Path, fmt.Sprint(entry.OldSize), fmt.Sprint(entry.NewSize),
			formatTime(entry.OldModTime), formatTime(entry.NewModTime),
		}); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

func statusMarker(status dircompare.Status) string {
	switch status {
	case dircompare.Added:
		return "+"
	case dircompare.Removed:
		return "−"
	case dircompare.Changed:
		return "~"
	default:
		return "="
	}
}

func formatSize(value int64, missing bool) string {
	if missing {
		return "—"
	}
	return fmt.Sprintf("%d B", value)
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}
