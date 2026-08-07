// Command ayame-icon generates platform packaging icons for release builds.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/ayame-editor/ayame-diff/internal/appicon"
)

func main() {
	out := flag.String("out", "packaging/icons", "output directory")
	flag.Parse()
	if err := appicon.WriteSet(*out); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
