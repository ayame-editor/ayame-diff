// Package htmlreport renders a diff result to a single self-contained HTML file
// (inline CSS, no external assets) — a shareable comparison report
// (hjosugi/ayame-diff#33). It reuses internal/worddiff for word-level
// highlighting, matching the web GUI's presentation.
package htmlreport

import (
	"bufio"
	"fmt"
	"html"
	"io"

	"github.com/hjosugi/ayame-diff/internal/linediff"
	"github.com/hjosugi/ayame-diff/internal/worddiff"
)

const style = `
:root{--bg:#fff;--fg:#1c2128;--muted:#6b7480;--border:#d7dce1;--panel:#f6f8fa;
--add-bg:#e6ffec;--del-bg:#ffebe9;--chg-bg:#fff5d6;--w-add:#a6f3b8;--w-del:#ffc2c0;}
@media(prefers-color-scheme:dark){:root{--bg:#0d1117;--fg:#e6edf3;--muted:#8b949e;
--border:#30363d;--panel:#161b22;--add-bg:#12261a;--del-bg:#2d1214;--chg-bg:#2b2413;
--w-add:#1a5a2a;--w-del:#6e2429;}}
*{box-sizing:border-box}body{margin:0;background:var(--bg);color:var(--fg);
font:14px/1.5 system-ui,-apple-system,"Segoe UI",Meiryo,sans-serif}
header{padding:.8rem 1rem;border-bottom:1px solid var(--border)}
h1{margin:0;font-size:1.1rem}.summary{padding:.6rem 1rem;color:var(--muted);
font-variant-numeric:tabular-nums}.summary b{color:var(--fg)}
main{padding:1rem;max-width:1200px;margin:0 auto}
.hunk{border:1px solid var(--border);border-radius:8px;overflow:hidden;margin-bottom:.8rem}
.hh{font:.78rem ui-monospace,Menlo,Consolas,monospace;padding:.35rem .7rem;
background:var(--panel);color:var(--muted);border-bottom:1px solid var(--border)}
.rows{overflow-x:auto}.row{display:grid;grid-template-columns:1fr 1fr}
.cell{display:grid;grid-template-columns:3rem 1fr;gap:.5rem;padding:.1rem .5rem;
font:.82rem ui-monospace,Menlo,Consolas,monospace;white-space:pre-wrap;
word-break:break-word;border-top:1px solid var(--border)}
.cell:first-child{border-right:1px solid var(--border)}
.ln{color:var(--muted);text-align:right;user-select:none}
.add{background:var(--add-bg)}.del{background:var(--del-bg)}.chg{background:var(--chg-bg)}
.w-add{background:var(--w-add);border-radius:3px}.w-del{background:var(--w-del);border-radius:3px}
`

// Write renders res to w as a full self-contained HTML document. title labels
// the report (e.g. "old.txt vs new.txt").
func Write(w io.Writer, old, new linediff.Lines, res linediff.Result, title string) error {
	bw := bufio.NewWriter(w)
	esc := html.EscapeString
	fmt.Fprintf(bw, "<!doctype html>\n<html><head><meta charset=\"utf-8\">"+
		"<meta name=\"viewport\" content=\"width=device-width,initial-scale=1\">"+
		"<title>ayame-diff report: %s</title><style>%s</style></head><body>\n", esc(title), style)
	fmt.Fprintf(bw, "<header><h1>ayame-diff</h1></header>\n")
	fmt.Fprintf(bw, "<div class=\"summary\">%s: <b>%d</b> hunks, <b>%d</b> added, <b>%d</b> deleted, <b>%d</b> modified</div>\n",
		esc(title), res.HunkCount, res.Added, res.Deleted, res.Modified)
	fmt.Fprintln(bw, "<main>")
	for _, h := range res.Hunks {
		writeHunk(bw, old, new, h)
	}
	if len(res.Hunks) == 0 {
		fmt.Fprintln(bw, `<p style="color:var(--muted)">No differences.</p>`)
	}
	fmt.Fprintln(bw, "</main></body></html>")
	return bw.Flush()
}

func writeHunk(bw *bufio.Writer, old, new linediff.Lines, h linediff.Hunk) {
	kind := kindTitle(h.Kind)
	fmt.Fprintf(bw, "<div class=\"hunk\"><div class=\"hh\">@@ -%d,%d +%d,%d %s @@</div><div class=\"rows\">\n",
		h.OldStart+1, h.OldLen, h.NewStart+1, h.NewLen, kind)
	switch h.Kind {
	case linediff.Insert:
		for k := uint64(0); k < h.NewLen; k++ {
			row(bw, emptyCell(), cell("add", h.NewStart+k+1, html.EscapeString(lineAt(new, h.NewStart+k))))
		}
	case linediff.Delete:
		for k := uint64(0); k < h.OldLen; k++ {
			row(bw, cell("del", h.OldStart+k+1, html.EscapeString(lineAt(old, h.OldStart+k))), emptyCell())
		}
	default: // Replace
		pairs := min(h.OldLen, h.NewLen)
		for k := uint64(0); k < pairs; k++ {
			ol, nl := lineAt(old, h.OldStart+k), lineAt(new, h.NewStart+k)
			lhs, rhs := ol, nl
			if wd, ok := worddiff.Diff(ol, nl); ok {
				lhs = markup(wd.Old, "w-del")
				rhs = markup(wd.New, "w-add")
			} else {
				lhs, rhs = html.EscapeString(ol), html.EscapeString(nl)
			}
			row(bw, cellHTML("chg", h.OldStart+k+1, lhs), cellHTML("chg", h.NewStart+k+1, rhs))
		}
		for k := pairs; k < h.OldLen; k++ {
			row(bw, cell("del", h.OldStart+k+1, html.EscapeString(lineAt(old, h.OldStart+k))), emptyCell())
		}
		for k := pairs; k < h.NewLen; k++ {
			row(bw, emptyCell(), cell("add", h.NewStart+k+1, html.EscapeString(lineAt(new, h.NewStart+k))))
		}
	}
	fmt.Fprintln(bw, "</div></div>")
}

// markup renders word-diff segments to HTML, wrapping changed segments.
func markup(segs []worddiff.Segment, cls string) string {
	out := ""
	for _, s := range segs {
		if s.Changed {
			out += `<span class="` + cls + `">` + html.EscapeString(s.Text) + `</span>`
		} else {
			out += html.EscapeString(s.Text)
		}
	}
	return out
}

func row(bw *bufio.Writer, left, right string) {
	fmt.Fprintf(bw, "<div class=\"row\">%s%s</div>\n", left, right)
}
func cell(cls string, ln uint64, escaped string) string { return cellHTML(cls, ln, escaped) }
func cellHTML(cls string, ln uint64, inner string) string {
	return fmt.Sprintf(`<div class="cell %s"><span class="ln">%d</span><span>%s</span></div>`, cls, ln, inner)
}
func emptyCell() string { return `<div class="cell"><span class="ln"></span><span></span></div>` }

func lineAt(l linediff.Lines, i uint64) string {
	s, ok := l.Line(i)
	if !ok {
		return ""
	}
	return s
}

func kindTitle(k linediff.Kind) string {
	switch k {
	case linediff.Insert:
		return "Insert"
	case linediff.Delete:
		return "Delete"
	case linediff.Replace:
		return "Replace"
	default:
		return "Unknown"
	}
}
