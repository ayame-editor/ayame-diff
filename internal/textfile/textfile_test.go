package textfile

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	xencoding "golang.org/x/text/encoding"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

func encodeText(t *testing.T, text string, enc xencoding.Encoding) []byte {
	t.Helper()
	out, _, err := transform.Bytes(enc.NewEncoder(), []byte(text))
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	return out
}

// TestReadWriteJapaneseBusinessData covers #278 end to end through the profile
// round-trip: read a typical Japanese business file (a CP932 CSV, a UTF-8 CSV
// that Excel wrote with a BOM, and an ISO-2022-JP mail), edit one line, and
// write it back with the encoding, BOM and terminator preserved rather than
// normalized to BOM-less UTF-8 with LF.
func TestReadWriteJapaneseBusinessData(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		raw     []byte
		wantEnc string
		wantBOM bool
		edit    func(lines []string) []string
		want    func(t *testing.T) []byte
	}{
		{
			name:    "CP932 CSV",
			raw:     encodeText(t, "商品コード,商品名,数量\nA-001,①りんご,10\nA-002,㈱テスト,3\n", japanese.ShiftJIS),
			wantEnc: "shift_jis",
			edit:    func(lines []string) []string { lines[1] = "A-001,①りんご,25"; return lines },
			want: func(t *testing.T) []byte {
				return encodeText(t, "商品コード,商品名,数量\nA-001,①りんご,25\nA-002,㈱テスト,3\n", japanese.ShiftJIS)
			},
		},
		{
			name:    "UTF-8 with BOM",
			raw:     append([]byte{0xEF, 0xBB, 0xBF}, []byte("商品,金額\nりんご,100\n")...),
			wantEnc: "utf-8",
			wantBOM: true,
			edit:    func(lines []string) []string { lines[1] = "りんご,150"; return lines },
			want: func(t *testing.T) []byte {
				return append([]byte{0xEF, 0xBB, 0xBF}, []byte("商品,金額\nりんご,150\n")...)
			},
		},
		{
			name:    "ISO-2022-JP mail",
			raw:     encodeText(t, "件名: 見積書の送付について\n担当: 田中\n", japanese.ISO2022JP),
			wantEnc: "iso-2022-jp",
			edit:    func(lines []string) []string { lines[1] = "担当: 鈴木"; return lines },
			want: func(t *testing.T) []byte {
				return encodeText(t, "件名: 見積書の送付について\n担当: 鈴木\n", japanese.ISO2022JP)
			},
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), "data")
			if err := os.WriteFile(path, test.raw, 0o644); err != nil {
				t.Fatal(err)
			}

			lines, profile, err := ReadAll(path, "auto", 1<<20)
			if err != nil {
				t.Fatalf("ReadAll: %v", err)
			}
			if profile.Encoding != test.wantEnc {
				t.Fatalf("profile.Encoding = %q, want %q", profile.Encoding, test.wantEnc)
			}
			if profile.BOM != test.wantBOM {
				t.Errorf("profile.BOM = %v, want %v", profile.BOM, test.wantBOM)
			}
			if profile.LineEnding != "\n" || !profile.FinalNewline {
				t.Errorf("profile line conventions = %q/final=%v, want \\n/final=true", profile.LineEnding, profile.FinalNewline)
			}

			if err := Write(path, test.edit(lines), profile); err != nil {
				t.Fatalf("Write: %v", err)
			}
			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if want := test.want(t); !bytes.Equal(got, want) {
				t.Fatalf("written bytes = % x, want % x", got, want)
			}
		})
	}
}
