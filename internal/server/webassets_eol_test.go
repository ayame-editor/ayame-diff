package server

import (
	"io/fs"
	"path"
	"strings"
	"testing"
)

// TestWebAssetsAreCheckedOutWithLF guards the whole class of bug that made
// TestWhitespaceMarkersBuiltOnlyWhenShown fail on Windows and nowhere else.
//
// The assets are embedded and served verbatim, so their bytes should not depend
// on the platform that built the binary. .gitattributes left .js/.css/.html
// under "text=auto", which checks them out as CRLF on Windows: the server then
// shipped different bytes there, and every test asserting "...;\n});" against an
// asset failed on that one platform. Because the assertions were spread across
// several files and only one CI job runs Windows, the cause was far from the
// symptom.
func TestWebAssetsAreCheckedOutWithLF(t *testing.T) {
	t.Parallel()

	var checked int
	err := fs.WalkDir(webFS, "web", func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		switch path.Ext(name) {
		case ".js", ".css", ".html":
		default:
			return nil
		}
		checked++
		// Read the raw bytes: readWebAsset normalizes line endings on purpose,
		// so going through it here would hide exactly what this test looks for.
		b, err := webFS.ReadFile(name)
		if err != nil {
			return err
		}
		if i := strings.Index(string(b), "\r\n"); i >= 0 {
			line := strings.Count(string(b[:i]), "\n") + 1
			t.Errorf("%s has CRLF line endings (first at line %d); .gitattributes should pin it to eol=lf, "+
				"and an existing clone needs a fresh checkout to pick that up", name, line)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if checked == 0 {
		t.Fatal("no web assets were checked; the walk found nothing")
	}
}
