package main

import (
	"bufio"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/hjosugi/ayame-diff/internal/hexdiff"
)

// runBin implements: ayame-diff bin [flags] OLD NEW
func runBin(args []string) {
	fs := flag.NewFlagSet("ayame-diff bin", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var maxRegions, maxBytes int
	fs.IntVar(&maxRegions, "max-regions", 256, "maximum differing regions to print")
	fs.IntVar(&maxBytes, "max-bytes", 32, "maximum bytes of hex shown per region side")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), `ayame-diff bin [flags] OLD NEW

Byte-level (binary/hex) diff of two files. Prints each differing region as its
offset with the old and new bytes in hex. Streams both files, so it stays
memory-bounded on large inputs.`)
		fmt.Fprintln(fs.Output(), "\nOptions:")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(2)
	}
	if fs.NArg() != 2 {
		fmt.Fprintln(os.Stderr, "error: bin needs exactly two files: OLD NEW")
		os.Exit(2)
	}

	res, err := hexdiff.Compare(fs.Arg(0), fs.Arg(1), hexdiff.Options{MaxRegions: maxRegions})
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(2)
	}

	if res.Equal {
		fmt.Fprintf(os.Stderr, "identical (%d bytes)\n", res.OldSize)
		return
	}

	bw := bufio.NewWriter(os.Stdout)
	for _, r := range res.Regions {
		fmt.Fprintf(bw, "@ 0x%08x\n", r.Offset)
		fmt.Fprintf(bw, "  - %s\n", hexBytes(r.Old, maxBytes))
		fmt.Fprintf(bw, "  + %s\n", hexBytes(r.New, maxBytes))
	}
	bw.Flush()

	suffix := ""
	if res.Truncated {
		suffix = " (truncated; raise --max-regions)"
	}
	fmt.Fprintf(os.Stderr, "files differ: %d region(s), %d byte(s); sizes %d / %d%s\n",
		len(res.Regions), res.TotalDiffBytes, res.OldSize, res.NewSize, suffix)
}

// hexBytes formats b as space-separated hex, capped at max bytes with a note.
func hexBytes(b []byte, max int) string {
	if len(b) == 0 {
		return "(none)"
	}
	shown := b
	extra := 0
	if max > 0 && len(b) > max {
		shown = b[:max]
		extra = len(b) - max
	}
	parts := make([]string, len(shown))
	for i, x := range shown {
		parts[i] = hex.EncodeToString([]byte{x})
	}
	out := strings.Join(parts, " ")
	if extra > 0 {
		out += fmt.Sprintf(" …(+%d)", extra)
	}
	return out
}
