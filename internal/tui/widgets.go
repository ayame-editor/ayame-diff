package tui

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

type Choice struct {
	Label       string
	Description string
}

type MultiSelectOptions struct {
	Initial        []bool
	MinSelected    int
	MaxSelected    int
	MaxSelectedSet bool
	IndexBase      int
	EmptyMessage   string
	AdditionalHelp string
}

func PromptLine(t Terminal, title, label, defaultValue, help string, validate func(string) error) (string, error) {
	runes := []rune(defaultValue)
	cursor := len(runes)
	status := ""

	for {
		width, height := MustSize(t)
		display := renderEditableLine(runes, cursor, maxInt(4, width-2))
		lines := []string{
			title,
			strings.Repeat("=", minInt(DisplayWidth(title), width)),
			"",
			label,
			"> " + display,
		}
		if status != "" {
			lines = append(lines, "", "Error: "+status)
		}
		lines = append(lines, "")
		if help != "" {
			lines = append(lines, help)
		}
		lines = append(lines, "Enter: continue   Esc: cancel   Ctrl+U: clear   Left/Right/Home/End: edit")
		if err := t.Draw(NormalizeScreen(strings.Join(lines, "\n"), width, height)); err != nil {
			return "", err
		}

		key, err := ReadRequiredKey(t)
		if err != nil {
			return "", err
		}
		status = ""
		switch key.Kind {
		case KeyEnter:
			value := string(runes)
			if validate != nil {
				if err := validate(value); err != nil {
					status = err.Error()
					continue
				}
			}
			return value, nil
		case KeyEscape:
			return "", ErrCancelled
		case KeyLeft:
			if cursor > 0 {
				cursor--
			}
		case KeyRight:
			if cursor < len(runes) {
				cursor++
			}
		case KeyHome:
			cursor = 0
		case KeyEnd:
			cursor = len(runes)
		case KeyBackspace:
			if cursor > 0 {
				runes = append(runes[:cursor-1], runes[cursor:]...)
				cursor--
			}
		case KeyDelete:
			if cursor < len(runes) {
				runes = append(runes[:cursor], runes[cursor+1:]...)
			}
		case KeyRune:
			if key.Ctrl {
				switch unicode.ToLower(key.Rune) {
				case 'a':
					cursor = 0
				case 'e':
					cursor = len(runes)
				case 'u':
					runes = runes[:0]
					cursor = 0
				case 'w':
					start := cursor
					for start > 0 && unicode.IsSpace(runes[start-1]) {
						start--
					}
					for start > 0 && !unicode.IsSpace(runes[start-1]) {
						start--
					}
					runes = append(runes[:start], runes[cursor:]...)
					cursor = start
				}
				continue
			}
			if key.Rune >= 0x20 && key.Rune != 0x7f && len(runes) < 32768 {
				runes = append(runes, 0)
				copy(runes[cursor+1:], runes[cursor:])
				runes[cursor] = key.Rune
				cursor++
			}
		}
	}
}

func SelectOne(t Terminal, title, message string, choices []Choice, initial int) (int, error) {
	if len(choices) == 0 {
		return -1, fmt.Errorf("selection has no choices")
	}
	cursor := initial
	if cursor < 0 || cursor >= len(choices) {
		cursor = 0
	}

	for {
		width, height := MustSize(t)
		messageLines := []string(nil)
		if message != "" {
			messageLines = strings.Split(message, "\n")
		}
		fixedRows := 3 + 2 // title block, blank line, and footer
		if len(messageLines) > 0 {
			fixedRows += len(messageLines) + 1
		}
		if height >= 12 {
			fixedRows++ // selected choice description
		}
		listRows := height - fixedRows
		if listRows < 3 {
			listRows = 3
		}
		start := scrollStart(cursor, len(choices), listRows)
		end := minInt(start+listRows, len(choices))

		lines := []string{title, strings.Repeat("=", minInt(DisplayWidth(title), width)), ""}
		if len(messageLines) > 0 {
			lines = append(lines, messageLines...)
			lines = append(lines, "")
		}
		for i := start; i < end; i++ {
			marker := "  "
			if i == cursor {
				marker = "> "
			}
			number := strconv.Itoa(i + 1)
			line := marker + number + ". " + choices[i].Label
			lines = append(lines, TruncateDisplay(line, width))
			if i == cursor && choices[i].Description != "" && height >= 12 {
				lines = append(lines, "     "+TruncateDisplay(choices[i].Description, maxInt(1, width-5)))
			}
		}
		lines = append(lines, "", "Up/Down: move   Enter/Space: select   Home/End: jump   Esc: cancel")
		if err := t.Draw(NormalizeScreen(strings.Join(lines, "\n"), width, height)); err != nil {
			return -1, err
		}

		key, err := ReadRequiredKey(t)
		if err != nil {
			return -1, err
		}
		switch key.Kind {
		case KeyUp:
			if cursor > 0 {
				cursor--
			} else {
				cursor = len(choices) - 1
			}
		case KeyDown:
			if cursor+1 < len(choices) {
				cursor++
			} else {
				cursor = 0
			}
		case KeyPageUp:
			cursor = maxInt(0, cursor-listRows)
		case KeyPageDown:
			cursor = minInt(len(choices)-1, cursor+listRows)
		case KeyHome:
			cursor = 0
		case KeyEnd:
			cursor = len(choices) - 1
		case KeyEnter:
			return cursor, nil
		case KeyRune:
			if key.Rune == ' ' {
				return cursor, nil
			}
			if key.Rune >= '1' && key.Rune <= '9' {
				index := int(key.Rune - '1')
				if index < len(choices) {
					cursor = index
				}
			}
		case KeyEscape:
			return -1, ErrCancelled
		}
	}
}

func Confirm(t Terminal, title, message string, defaultYes bool) (bool, error) {
	initial := 1
	if defaultYes {
		initial = 0
	}
	index, err := SelectOne(t, title, message, []Choice{{Label: "Yes"}, {Label: "No"}}, initial)
	if err != nil {
		return false, err
	}
	return index == 0, nil
}

func MultiSelect(t Terminal, title, message string, items []string, options MultiSelectOptions) ([]bool, error) {
	if len(items) == 0 {
		return nil, fmt.Errorf("multi-selection has no items")
	}
	selected := make([]bool, len(items))
	if len(options.Initial) == len(items) {
		copy(selected, options.Initial)
	}
	filter := ""
	cursor := 0
	status := ""

	for {
		visible := filterIndexes(items, filter)
		if cursor >= len(visible) {
			cursor = maxInt(0, len(visible)-1)
		}

		width, height := MustSize(t)
		listRows := height - 11
		if message == "" {
			listRows++
		}
		if options.AdditionalHelp != "" {
			listRows--
		}
		if listRows < 3 {
			listRows = 3
		}
		start := scrollStart(cursor, len(visible), listRows)
		end := minInt(start+listRows, len(visible))

		count := countSelected(selected)
		lines := []string{
			title,
			strings.Repeat("=", minInt(DisplayWidth(title), width)),
			"",
		}
		if message != "" {
			lines = append(lines, message)
		}
		filterText := "(none)"
		if filter != "" {
			filterText = filter
		}
		lines = append(lines,
			fmt.Sprintf("Selected: %d / %d   Visible: %d   Filter: %s", count, len(items), len(visible), filterText),
			"",
		)

		if len(visible) == 0 {
			empty := options.EmptyMessage
			if empty == "" {
				empty = "No columns match the current filter."
			}
			lines = append(lines, empty)
		} else {
			for pos := start; pos < end; pos++ {
				index := visible[pos]
				marker := "  "
				if pos == cursor {
					marker = "> "
				}
				checked := "[ ]"
				if selected[index] {
					checked = "[x]"
				}
				number := fmt.Sprintf("%6d", index+options.IndexBase)
				prefix := marker + checked + " " + number + "  "
				lines = append(lines, prefix+TruncateDisplay(InlineText(items[index]), maxInt(1, width-DisplayWidth(prefix))))
			}
		}
		if status != "" {
			lines = append(lines, "", "Error: "+status)
		} else {
			lines = append(lines, "")
		}
		lines = append(lines, "Space: toggle   A: select visible   N: clear visible   I: invert visible")
		lines = append(lines, "/: search   C: clear search   Enter: confirm   Esc: cancel")
		if options.AdditionalHelp != "" {
			lines = append(lines, options.AdditionalHelp)
		}
		if err := t.Draw(NormalizeScreen(strings.Join(lines, "\n"), width, height)); err != nil {
			return nil, err
		}

		key, err := ReadRequiredKey(t)
		if err != nil {
			return nil, err
		}
		status = ""
		switch key.Kind {
		case KeyUp:
			if len(visible) > 0 {
				if cursor > 0 {
					cursor--
				} else {
					cursor = len(visible) - 1
				}
			}
		case KeyDown:
			if len(visible) > 0 {
				if cursor+1 < len(visible) {
					cursor++
				} else {
					cursor = 0
				}
			}
		case KeyPageUp:
			cursor = maxInt(0, cursor-listRows)
		case KeyPageDown:
			if len(visible) > 0 {
				cursor = minInt(len(visible)-1, cursor+listRows)
			}
		case KeyHome:
			cursor = 0
		case KeyEnd:
			if len(visible) > 0 {
				cursor = len(visible) - 1
			}
		case KeyEnter:
			count := countSelected(selected)
			if options.MinSelected > 0 && count < options.MinSelected {
				status = fmt.Sprintf("select at least %d column(s)", options.MinSelected)
				continue
			}
			if options.MaxSelectedSet && count > options.MaxSelected {
				status = fmt.Sprintf("select at most %d column(s)", options.MaxSelected)
				continue
			}
			result := make([]bool, len(selected))
			copy(result, selected)
			return result, nil
		case KeyEscape:
			return nil, ErrCancelled
		case KeyRune:
			switch unicode.ToLower(key.Rune) {
			case ' ':
				if len(visible) > 0 {
					index := visible[cursor]
					if selected[index] {
						selected[index] = false
					} else if options.MaxSelectedSet && countSelected(selected) >= options.MaxSelected {
						status = fmt.Sprintf("at most %d column(s) may be selected", options.MaxSelected)
					} else {
						selected[index] = true
					}
				}
			case 'a':
				reachedLimit := false
				for _, index := range visible {
					if selected[index] {
						continue
					}
					if options.MaxSelectedSet && countSelected(selected) >= options.MaxSelected {
						reachedLimit = true
						break
					}
					selected[index] = true
				}
				if reachedLimit {
					status = fmt.Sprintf("maximum of %d column(s) reached", options.MaxSelected)
				}
			case 'n':
				for _, index := range visible {
					selected[index] = false
				}
			case 'i':
				candidate := append([]bool(nil), selected...)
				for _, index := range visible {
					candidate[index] = !candidate[index]
				}
				if options.MaxSelectedSet && countSelected(candidate) > options.MaxSelected {
					status = fmt.Sprintf("inversion would exceed %d selected column(s)", options.MaxSelected)
				} else {
					selected = candidate
				}
			case 'c':
				filter = ""
				cursor = 0
			case '/':
				value, promptErr := PromptLine(t, title+" - Search", "Header substring (empty means all columns)", filter,
					"The search is case-insensitive. Press Esc to keep the previous filter.", nil)
				if promptErr != nil {
					if promptErr == ErrCancelled {
						continue
					}
					return nil, promptErr
				}
				filter = strings.TrimSpace(value)
				cursor = 0
			}
		}
	}
}

func ShowMessage(t Terminal, title string, lines []string) error {
	for {
		width, height := MustSize(t)
		screen := []string{title, strings.Repeat("=", minInt(DisplayWidth(title), width)), ""}
		screen = append(screen, lines...)
		screen = append(screen, "", "Enter: continue   Esc: cancel")
		if err := t.Draw(NormalizeScreen(strings.Join(screen, "\n"), width, height)); err != nil {
			return err
		}
		key, err := ReadRequiredKey(t)
		if err != nil {
			return err
		}
		switch key.Kind {
		case KeyEnter:
			return nil
		case KeyEscape:
			return ErrCancelled
		}
	}
}

func renderEditableLine(runes []rune, cursor, maxWidth int) string {
	if cursor < 0 {
		cursor = 0
	}
	if cursor > len(runes) {
		cursor = len(runes)
	}
	if maxWidth < 4 {
		return "|"
	}

	leftReserve := maxWidth * 2 / 3
	start := cursor
	used := 1 // cursor marker
	for start > 0 {
		width := runeDisplayWidth(runes[start-1])
		if used+width > leftReserve {
			break
		}
		start--
		used += width
	}
	if start > 0 {
		for used+3 > maxWidth && start < cursor {
			used -= runeDisplayWidth(runes[start])
			start++
		}
		used += 3
	}

	var b strings.Builder
	if start > 0 {
		b.WriteString("...")
	}
	for i := start; i < cursor; i++ {
		b.WriteRune(runes[i])
	}
	b.WriteRune('|')
	remaining := maxWidth - DisplayWidth(b.String())
	end := cursor
	for end < len(runes) {
		width := runeDisplayWidth(runes[end])
		reserveSuffix := 0
		if end+1 < len(runes) {
			reserveSuffix = 3
		}
		if width+reserveSuffix > remaining {
			break
		}
		b.WriteRune(runes[end])
		remaining -= width
		end++
	}
	if end < len(runes) && remaining >= 3 {
		b.WriteString("...")
	}
	return b.String()
}

func filterIndexes(items []string, filter string) []int {
	result := make([]int, 0, len(items))
	needle := strings.ToLower(strings.TrimSpace(filter))
	for i, item := range items {
		if needle == "" || strings.Contains(strings.ToLower(item), needle) || strings.Contains(strings.ToLower(InlineText(item)), needle) {
			result = append(result, i)
		}
	}
	return result
}

func countSelected(selected []bool) int {
	count := 0
	for _, value := range selected {
		if value {
			count++
		}
	}
	return count
}

func scrollStart(cursor, total, rows int) int {
	if total <= rows || rows <= 0 {
		return 0
	}
	start := cursor - rows/2
	if start < 0 {
		start = 0
	}
	maximum := total - rows
	if start > maximum {
		start = maximum
	}
	return start
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
