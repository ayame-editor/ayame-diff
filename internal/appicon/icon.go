// Package appicon renders the Ayame Diff flower mark and writes native icon
// containers without external image tooling.
package appicon

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
)

type petal struct {
	cx, cy, rx, ry, rotation float64
	color                    color.NRGBA
}

var petals = []petal{
	{32, 18, 6, 13.2, 0, color.NRGBA{0xa9, 0x92, 0xe0, 0xff}},
	{23, 27, 5.8, 13, -0.58, color.NRGBA{0x9b, 0x82, 0xd8, 0xff}},
	{41, 27, 5.8, 13, 0.58, color.NRGBA{0x79, 0x5f, 0xc3, 0xff}},
	{24, 43, 6.2, 13.5, 0.47, color.NRGBA{0x8e, 0x73, 0xcf, 0xff}},
	{40, 43, 6.2, 13.5, -0.47, color.NRGBA{0x6f, 0x56, 0xb8, 0xff}},
	{32, 38, 6.4, 11.8, 0, color.NRGBA{0x67, 0x4f, 0xaf, 0xff}},
}

// Render returns a square, transparent application icon. Supersampling keeps
// the mark legible in 16-pixel launchers and title bars.
func Render(size int) *image.NRGBA {
	if size < 1 {
		size = 1
	}
	dst := image.NewNRGBA(image.Rect(0, 0, size, size))
	const samples = 4
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			var out [4]float64
			for _, p := range petals {
				hits := 0
				for sy := 0; sy < samples; sy++ {
					for sx := 0; sx < samples; sx++ {
						fx := (float64(x) + (float64(sx)+.5)/samples) * 64 / float64(size)
						fy := (float64(y) + (float64(sy)+.5)/samples) * 64 / float64(size)
						gx := 32 + (fx-32)/1.40
						gy := 32 + (fy-32-1.5)/.95
						if inside(gx, gy, p) {
							hits++
						}
					}
				}
				blend(&out, p.color, float64(hits)/(samples*samples))
			}
			// Paired veins distinguish Diff from Editor while preserving the iris.
			fx := (float64(x) + .5) * 64 / float64(size)
			fy := (float64(y) + .5) * 64 / float64(size)
			if distanceToSegment(fx, fy, 25.5, 27, 32, 31) <= 1.15 {
				blend(&out, color.NRGBA{0x39, 0xa6, 0x6b, 0xff}, .95)
			}
			if distanceToSegment(fx, fy, 38.5, 27, 32, 31) <= 1.15 {
				blend(&out, color.NRGBA{0xe0, 0x6b, 0x7e, 0xff}, .95)
			}
			dst.SetNRGBA(x, y, color.NRGBA{
				uint8(math.Round(out[0] * 255)), uint8(math.Round(out[1] * 255)),
				uint8(math.Round(out[2] * 255)), uint8(math.Round(out[3] * 255)),
			})
		}
	}
	return dst
}

func inside(x, y float64, p petal) bool {
	s, c := math.Sincos(p.rotation)
	dx, dy := x-p.cx, y-p.cy
	rx := (dx*c + dy*s) / p.rx
	ry := (-dx*s + dy*c) / p.ry
	return rx*rx+ry*ry <= 1
}

func blend(dst *[4]float64, c color.NRGBA, coverage float64) {
	if coverage <= 0 {
		return
	}
	a := float64(c.A) / 255 * coverage
	outA := a + dst[3]*(1-a)
	if outA == 0 {
		return
	}
	rgb := [3]float64{float64(c.R) / 255, float64(c.G) / 255, float64(c.B) / 255}
	for i := range rgb {
		dst[i] = (rgb[i]*a + dst[i]*dst[3]*(1-a)) / outA
	}
	dst[3] = outA
}

func distanceToSegment(x, y, x1, y1, x2, y2 float64) float64 {
	dx, dy := x2-x1, y2-y1
	t := ((x-x1)*dx + (y-y1)*dy) / (dx*dx + dy*dy)
	t = math.Max(0, math.Min(1, t))
	return math.Hypot(x-(x1+t*dx), y-(y1+t*dy))
}

func encodePNG(size int) ([]byte, error) {
	var b bytes.Buffer
	err := png.Encode(&b, Render(size))
	return b.Bytes(), err
}

// WriteSet creates the desktop PNG, Windows ICO and macOS ICNS icon set.
func WriteSet(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	png256, err := encodePNG(256)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "ayame-diff.png"), png256, 0o644); err != nil {
		return err
	}
	if err := writeICO(filepath.Join(dir, "ayame-diff.ico")); err != nil {
		return err
	}
	return writeICNS(filepath.Join(dir, "ayame-diff.icns"))
}

func writeICO(path string) error {
	sizes := []int{16, 32, 48, 64, 128, 256}
	images := make([][]byte, len(sizes))
	for i, size := range sizes {
		var err error
		images[i], err = encodePNG(size)
		if err != nil {
			return err
		}
	}
	var out bytes.Buffer
	_ = binary.Write(&out, binary.LittleEndian, uint16(0))
	_ = binary.Write(&out, binary.LittleEndian, uint16(1))
	_ = binary.Write(&out, binary.LittleEndian, uint16(len(images)))
	offset := 6 + len(images)*16
	for i, data := range images {
		width := byte(sizes[i])
		if sizes[i] == 256 {
			width = 0
		}
		out.Write([]byte{width, width, 0, 0})
		_ = binary.Write(&out, binary.LittleEndian, uint16(1))
		_ = binary.Write(&out, binary.LittleEndian, uint16(32))
		_ = binary.Write(&out, binary.LittleEndian, uint32(len(data)))
		_ = binary.Write(&out, binary.LittleEndian, uint32(offset))
		offset += len(data)
	}
	for _, data := range images {
		out.Write(data)
	}
	return os.WriteFile(path, out.Bytes(), 0o644)
}

func writeICNS(path string) error {
	types := []string{"icp4", "icp5", "icp6", "ic07", "ic08", "ic09", "ic10"}
	sizes := []int{16, 32, 64, 128, 256, 512, 1024}
	var body bytes.Buffer
	for i, kind := range types {
		data, err := encodePNG(sizes[i])
		if err != nil {
			return err
		}
		body.WriteString(kind)
		_ = binary.Write(&body, binary.BigEndian, uint32(len(data)+8))
		body.Write(data)
	}
	var out bytes.Buffer
	out.WriteString("icns")
	_ = binary.Write(&out, binary.BigEndian, uint32(body.Len()+8))
	out.Write(body.Bytes())
	return os.WriteFile(path, out.Bytes(), 0o644)
}
