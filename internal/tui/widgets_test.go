package tui

import (
	"errors"
	"strings"
	"testing"
)

type fakeTerminal struct {
	keys    []Key
	screens []string
	width   int
	height  int
}

func (f *fakeTerminal) ReadKey() (Key, error) {
	if len(f.keys) == 0 {
		return Key{}, errors.New("no fake keys remain")
	}
	key := f.keys[0]
	f.keys = f.keys[1:]
	return key, nil
}
func (f *fakeTerminal) Size() (int, int) {
	if f.width == 0 {
		return 80, 24
	}
	return f.width, f.height
}
func (f *fakeTerminal) Draw(screen string) error {
	f.screens = append(f.screens, screen)
	return nil
}
func (f *fakeTerminal) Close() error { return nil }

func TestDisplayWidthJapanese(t *testing.T) {
	t.Parallel()
	if got := DisplayWidth("A東京"); got != 5 {
		t.Fatalf("DisplayWidth = %d, want 5", got)
	}
	if got := DisplayWidth("e\u0301"); got != 1 {
		t.Fatalf("combining DisplayWidth = %d, want 1", got)
	}
}

func TestInlineTextEscapesHeaderControls(t *testing.T) {
	t.Parallel()
	got := InlineText("name\tline\nnext\x01")
	if got != `name\tline\nnext\u0001` {
		t.Fatalf("InlineText = %q", got)
	}
}

func TestRenderEditableLineKeepsCursorVisible(t *testing.T) {
	t.Parallel()
	runes := []rune(`C:\very\long\日本語\directory\input.tsv`)
	got := renderEditableLine(runes, len(runes), 22)
	if !strings.Contains(got, "|") {
		t.Fatalf("cursor missing: %q", got)
	}
	if DisplayWidth(got) > 22 {
		t.Fatalf("display width %d > 22: %q", DisplayWidth(got), got)
	}
	if !strings.HasPrefix(got, "...") {
		t.Fatalf("long line should show a leading ellipsis: %q", got)
	}
}

func TestMultiSelectSearchAndSpace(t *testing.T) {
	t.Parallel()
	terminal := &fakeTerminal{keys: []Key{
		{Kind: KeyRune, Rune: '/'},
		{Kind: KeyRune, Rune: 'u'},
		{Kind: KeyRune, Rune: 'p'},
		{Kind: KeyRune, Rune: 'd'},
		{Kind: KeyEnter},
		{Kind: KeyRune, Rune: ' '},
		{Kind: KeyRune, Rune: 'c'},
		{Kind: KeyEnd},
		{Kind: KeyRune, Rune: ' '},
		{Kind: KeyEnter},
	}}
	selected, err := MultiSelect(terminal, "Select", "", []string{"id", "updated_at", "value"}, MultiSelectOptions{})
	if err != nil {
		t.Fatal(err)
	}
	want := []bool{false, true, true}
	for i := range want {
		if selected[i] != want[i] {
			t.Fatalf("selected = %#v, want %#v", selected, want)
		}
	}
}

func TestMultiSelectEnforcesMaximumImmediately(t *testing.T) {
	t.Parallel()
	terminal := &fakeTerminal{keys: []Key{
		{Kind: KeyRune, Rune: ' '},
		{Kind: KeyEnter},
	}}
	selected, err := MultiSelect(terminal, "Exclude", "", []string{"only_column"}, MultiSelectOptions{
		MaxSelected:    0,
		MaxSelectedSet: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if selected[0] {
		t.Fatalf("selected = %#v, want unselected", selected)
	}
	foundError := false
	for _, screen := range terminal.screens {
		if strings.Contains(screen, "at most 0") {
			foundError = true
			break
		}
	}
	if !foundError {
		t.Fatal("maximum-selection message was not rendered")
	}
}

func TestMultiSelectSelectVisibleStopsAtMaximum(t *testing.T) {
	t.Parallel()
	terminal := &fakeTerminal{keys: []Key{
		{Kind: KeyRune, Rune: 'a'},
		{Kind: KeyEnter},
	}}
	selected, err := MultiSelect(terminal, "Exclude", "", []string{"a", "b", "c"}, MultiSelectOptions{
		MaxSelected:    2,
		MaxSelectedSet: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := countSelected(selected); got != 2 {
		t.Fatalf("selected count = %d, want 2: %#v", got, selected)
	}
}

func TestFilterIndexesUnicodeCaseInsensitive(t *testing.T) {
	t.Parallel()
	got := filterIndexes([]string{"CustomerID", "更新日時", "Value"}, "customer")
	if len(got) != 1 || got[0] != 0 {
		t.Fatalf("filterIndexes = %#v", got)
	}
}

func TestFilterIndexesMatchesEscapedControlText(t *testing.T) {
	t.Parallel()
	got := filterIndexes([]string{"normal", "multi\nline"}, `\n`)
	if len(got) != 1 || got[0] != 1 {
		t.Fatalf("filterIndexes = %#v", got)
	}
}
