package appicon

import (
	"encoding/binary"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestMarkIsSquareAndFillsCanvas(t *testing.T) {
	img := Render(64)
	x0, y0, x1, y1 := 64, 64, 0, 0
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			if img.NRGBAAt(x, y).A > 8 {
				x0, y0 = min(x0, x), min(y0, y)
				x1, y1 = max(x1, x), max(y1, y)
			}
		}
	}
	w, h := x1-x0+1, y1-y0+1
	if aspect := float64(w) / float64(h); aspect < .9 || aspect > 1.12 {
		t.Fatalf("mark aspect drifted: %dx%d = %.2f", w, h, aspect)
	}
	if w < 44 || h < 44 {
		t.Fatalf("mark too small at launcher sizes: %dx%d", w, h)
	}
}

func TestWriteSetProducesValidContainers(t *testing.T) {
	dir := t.TempDir()
	if err := WriteSet(dir); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(filepath.Join(dir, "ayame-diff.png"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := png.Decode(f); err != nil {
		t.Fatalf("PNG: %v", err)
	}
	ico, err := os.ReadFile(filepath.Join(dir, "ayame-diff.ico"))
	if err != nil || len(ico) < 6 || binary.LittleEndian.Uint16(ico[2:4]) != 1 || binary.LittleEndian.Uint16(ico[4:6]) != 6 {
		t.Fatalf("ICO header: %v", err)
	}
	icns, err := os.ReadFile(filepath.Join(dir, "ayame-diff.icns"))
	if err != nil || len(icns) < 8 || string(icns[:4]) != "icns" || int(binary.BigEndian.Uint32(icns[4:8])) != len(icns) {
		t.Fatalf("ICNS header: %v", err)
	}
}
