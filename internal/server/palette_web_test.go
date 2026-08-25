package server

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// The diff palette is a system, not a set of independent picks: the semantic
// washes have to stay readable under body text, the semantic text has to stay
// readable on its own wash, and addition and deletion have to carry the same
// visual weight so neither side of a comparison shouts. The previous palette
// failed the first rule (--add-fg on --add-bg was 4.24:1, under AA) and the
// third, which is what made it read muddy over the warm ground.
func TestDiffPaletteIsReadableAndBalanced(t *testing.T) {
	t.Parallel()

	tokens := readWebAsset(t, "tokens.css")
	for _, theme := range []struct {
		name  string
		block string
	}{
		{"light", blockAfter(t, tokens, ":root {")},
		{"dark", blockAfter(t, tokens, `:root[data-theme="dark"] {`)},
	} {
		t.Run(theme.name, func(t *testing.T) {
			light := declarations(blockAfter(t, tokens, ":root {"))
			values := declarations(theme.block)
			// The dark block overrides part of the palette; everything it does
			// not restate comes from :root, exactly as the cascade resolves it.
			for name, value := range light {
				if _, ok := values[name]; !ok {
					values[name] = value
				}
			}

			bg := mustColor(t, resolve(t, values, "--bg"))
			body := mustColor(t, resolve(t, values, "--fg"))

			washes := map[string]rgb{}
			for _, name := range []string{"--add-bg", "--del-bg", "--chg-bg"} {
				washes[name] = compositeWash(t, values, name, bg)
			}
			for name, wash := range washes {
				if got := contrastRatio(body, wash); got < 7 {
					t.Errorf("body text on %s is %.2f:1, want at least 7:1", name, got)
				}
			}
			for _, pair := range [][2]string{{"--add-fg", "--add-bg"}, {"--del-fg", "--del-bg"}, {"--chg-fg", "--chg-bg"}} {
				text := mustColor(t, resolve(t, values, pair[0]))
				if got := contrastRatio(text, washes[pair[1]]); got < 4.5 {
					t.Errorf("%s on %s is %.2f:1, under the 4.5:1 minimum", pair[0], pair[1], got)
				}
				if got := contrastRatio(text, bg); got < 4.5 {
					t.Errorf("%s on the page background is %.2f:1, under the 4.5:1 minimum", pair[0], got)
				}
			}

			// Balance: addition and deletion must not differ in weight, or the
			// eye reads one side of every comparison as the important one.
			addWash, delWash := relativeLuminance(washes["--add-bg"]), relativeLuminance(washes["--del-bg"])
			if math.Abs(addWash-delWash) > 0.03 {
				t.Errorf("the addition and deletion washes differ in luminance by %.3f, want at most 0.030", math.Abs(addWash-delWash))
			}
			addText := relativeLuminance(mustColor(t, resolve(t, values, "--add-fg")))
			delText := relativeLuminance(mustColor(t, resolve(t, values, "--del-fg")))
			if math.Abs(addText-delText) > 0.06 {
				t.Errorf("the addition and deletion text colors differ in luminance by %.3f, want at most 0.060", math.Abs(addText-delText))
			}

			// Word highlights are painted over the change wash, so that is where
			// their color has to survive: a highlight that sinks into the row
			// beneath it is the muddiness this palette was rebuilt to remove.
			for _, name := range []string{"--word-add", "--word-del"} {
				word := compositeWash(t, values, name, washes["--chg-bg"])
				if got := contrastRatio(body, word); got < 4.5 {
					t.Errorf("body text on %s over the change row is %.2f:1, under 4.5:1", name, got)
				}
				if got := saturationDistance(word, washes["--chg-bg"]); got < 0.10 {
					t.Errorf("%s is only %.3f from the change row it sits on; it will read as mud", name, got)
				}
			}
		})
	}
}

// blockAfter returns the declarations of the first CSS block opened by marker.
func blockAfter(t *testing.T, css, marker string) string {
	t.Helper()
	start := strings.Index(css, marker)
	if start < 0 {
		t.Fatalf("tokens.css has no %q block", marker)
	}
	start += len(marker)
	end := strings.Index(css[start:], "}")
	if end < 0 {
		t.Fatalf("the %q block is never closed", marker)
	}
	return css[start : start+end]
}

var declarationPattern = regexp.MustCompile(`(--[a-z-]+)\s*:\s*([^;]+);`)

func declarations(block string) map[string]string {
	out := map[string]string{}
	for _, match := range declarationPattern.FindAllStringSubmatch(block, -1) {
		out[match[1]] = strings.TrimSpace(match[2])
	}
	return out
}

// resolve follows var() references until it reaches a literal.
func resolve(t *testing.T, values map[string]string, name string) string {
	t.Helper()
	value, ok := values[name]
	if !ok {
		t.Fatalf("tokens.css does not define %s", name)
	}
	for i := 0; strings.HasPrefix(value, "var("); i++ {
		if i > 8 {
			t.Fatalf("%s resolves in a loop", name)
		}
		inner := strings.TrimSuffix(strings.TrimPrefix(value, "var("), ")")
		value = resolve(t, values, strings.TrimSpace(inner))
	}
	return value
}

var mixPattern = regexp.MustCompile(`color-mix\(in srgb,\s*(.+?)\s+([0-9.]+)%,\s*transparent\)`)

// compositeWash flattens a translucent token over the color behind it, which is
// what the browser paints and therefore what readability depends on.
func compositeWash(t *testing.T, values map[string]string, name string, behind rgb) rgb {
	t.Helper()
	value := resolve(t, values, name)
	match := mixPattern.FindStringSubmatch(value)
	if match == nil {
		return mustColor(t, value)
	}
	source := strings.TrimSpace(match[1])
	if strings.HasPrefix(source, "var(") {
		source = resolve(t, values, strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(source, "var("), ")")))
	}
	front := mustColor(t, source)
	alpha, err := strconv.ParseFloat(match[2], 64)
	if err != nil {
		t.Fatalf("%s has an unreadable percentage: %v", name, err)
	}
	alpha /= 100
	return rgb{
		r: front.r*alpha + behind.r*(1-alpha),
		g: front.g*alpha + behind.g*(1-alpha),
		b: front.b*alpha + behind.b*(1-alpha),
	}
}

type rgb struct{ r, g, b float64 }

func mustColor(t *testing.T, value string) rgb {
	t.Helper()
	color, err := parseHex(value)
	if err != nil {
		t.Fatalf("%q: %v", value, err)
	}
	return color
}

func parseHex(value string) (rgb, error) {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "#") || (len(value) != 7 && len(value) != 4) {
		return rgb{}, fmt.Errorf("not a hex color")
	}
	digits := value[1:]
	if len(digits) == 3 {
		digits = string([]byte{digits[0], digits[0], digits[1], digits[1], digits[2], digits[2]})
	}
	parsed, err := strconv.ParseUint(digits, 16, 32)
	if err != nil {
		return rgb{}, err
	}
	return rgb{
		r: float64((parsed >> 16) & 0xff),
		g: float64((parsed >> 8) & 0xff),
		b: float64(parsed & 0xff),
	}, nil
}

func channelLuminance(value float64) float64 {
	v := value / 255
	if v <= 0.04045 {
		return v / 12.92
	}
	return math.Pow((v+0.055)/1.055, 2.4)
}

func relativeLuminance(c rgb) float64 {
	return 0.2126*channelLuminance(c.r) + 0.7152*channelLuminance(c.g) + 0.0722*channelLuminance(c.b)
}

func contrastRatio(a, b rgb) float64 {
	first, second := relativeLuminance(a), relativeLuminance(b)
	if first < second {
		first, second = second, first
	}
	return (first + 0.05) / (second + 0.05)
}

// saturationDistance measures how far apart two colors are once lightness is
// taken out, which is what "does this highlight still look colored" means.
func saturationDistance(a, b rgb) float64 {
	an, bn := normalize(a), normalize(b)
	return math.Sqrt(math.Pow(an.r-bn.r, 2)+math.Pow(an.g-bn.g, 2)+math.Pow(an.b-bn.b, 2)) / 255
}

func normalize(c rgb) rgb {
	mean := (c.r + c.g + c.b) / 3
	if mean == 0 {
		return c
	}
	scale := 128 / mean
	return rgb{r: c.r * scale, g: c.g * scale, b: c.b * scale}
}
