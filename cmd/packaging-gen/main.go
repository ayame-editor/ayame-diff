// Command packaging-gen creates package-manager manifests for a release.
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/ayame-editor/ayame-diff/internal/packaging"
)

func main() {
	version := flag.String("version", "", "release version (vX.Y.Z)")
	checksums := flag.String("checksums", "release/SHA256SUMS", "SHA256SUMS path")
	out := flag.String("out", "dist/packaging", "output directory")
	date := flag.String("date", time.Now().UTC().Format("2006-01-02"), "release date (YYYY-MM-DD)")
	flag.Parse()
	parsed, err := time.Parse("2006-01-02", *date)
	if err == nil {
		err = packaging.Generate(*version, *checksums, *out, parsed)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
