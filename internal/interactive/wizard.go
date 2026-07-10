package interactive

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/hjosugi/fcsv-diff/internal/engine"
	"github.com/hjosugi/fcsv-diff/internal/tui"
)

var ErrCancelled = tui.ErrCancelled
var ErrInterrupted = tui.ErrInterrupted

type keyMode uint8

const (
	keyModeAll keyMode = iota
	keyModeInclude
	keyModeExclude
)

type wizard struct {
	terminal tui.Terminal
	cfg      engine.Config
	title    string
}

func Run(cfg engine.Config, version string) (result engine.Config, resultErr error) {
	terminal, err := tui.Open()
	if err != nil {
		return cfg, err
	}
	w := &wizard{
		terminal: terminal,
		cfg:      cfg,
		title:    "fcsv-diff " + version + " - Interactive setup",
	}
	defer func() {
		if closeErr := terminal.Close(); resultErr == nil && closeErr != nil {
			resultErr = closeErr
		}
	}()
	return w.run()
}

func (w *wizard) run() (engine.Config, error) {
	if err := w.editFiles(); err != nil {
		return w.cfg, err
	}
	inspection, err := w.inspectWithRecovery()
	if err != nil {
		return w.cfg, err
	}
	if err := w.editKeys(inspection); err != nil {
		return w.cfg, err
	}

	for {
		choice, err := tui.SelectOne(w.terminal, w.title+" - Review", w.reviewText(inspection), []tui.Choice{
			{Label: "Start comparison", Description: "Use the settings shown above and start processing."},
			{Label: "Edit key columns", Description: "Choose all, include selected headers, or exclude selected headers."},
			{Label: "Edit files and header settings", Description: "Change input/output paths and whether the first record is a header."},
			{Label: "Edit CSV/TSV parsing settings", Description: "Override format, delimiter, or safe/fast parser mode."},
			{Label: "Edit performance settings", Description: "Tune memory, temporary storage, partitions, and workers."},
			{Label: "Cancel"},
		}, 0)
		if err != nil {
			return w.cfg, err
		}
		switch choice {
		case 0:
			candidate := cloneConfig(w.cfg)
			if err := candidate.Validate(); err != nil {
				if msgErr := tui.ShowMessage(w.terminal, w.title+" - Invalid settings", []string{err.Error(), "", "Edit the relevant setting and try again."}); msgErr != nil {
					return w.cfg, msgErr
				}
				continue
			}
			return w.cfg, nil
		case 1:
			if err := w.editKeys(inspection); err != nil {
				return w.cfg, err
			}
		case 2:
			if err := w.editFiles(); err != nil {
				return w.cfg, err
			}
			inspection, err = w.inspectWithRecovery()
			if err != nil {
				return w.cfg, err
			}
			if err := w.editKeys(inspection); err != nil {
				return w.cfg, err
			}
		case 3:
			if err := w.editParsing(); err != nil {
				return w.cfg, err
			}
			inspection, err = w.inspectWithRecovery()
			if err != nil {
				return w.cfg, err
			}
			if err := w.editKeys(inspection); err != nil {
				return w.cfg, err
			}
		case 4:
			if err := w.editPerformance(); err != nil {
				return w.cfg, err
			}
		case 5:
			return w.cfg, ErrCancelled
		}
	}
}

func (w *wizard) editFiles() error {
	left, err := tui.PromptLine(w.terminal, w.title+" - Files", "Left / old CSV or TSV path", w.cfg.LeftPath,
		"You can drag and drop a file into the Windows console.", validateInputPath)
	if err != nil {
		return err
	}
	left = normalizePathInput(left)

	right, err := tui.PromptLine(w.terminal, w.title+" - Files", "Right / new CSV or TSV path", w.cfg.RightPath,
		"Row order may differ. Header order may also differ when alignment by name is enabled.", validateInputPath)
	if err != nil {
		return err
	}
	right = normalizePathInput(right)

	outputDefault := w.cfg.OutputPath
	if outputDefault == "" {
		outputDefault = defaultOutputPath(left)
	}
	output, err := tui.PromptLine(w.terminal, w.title+" - Files", "Difference output path (TSV or TSV.GZ)", outputDefault,
		"Use a .gz suffix for gzip output.", func(value string) error {
			return validateOutputPath(value, left, right)
		})
	if err != nil {
		return err
	}
	output = normalizePathInput(output)

	headerInitial := 0
	if !w.cfg.HasHeader {
		headerInitial = 1
	}
	headerChoice, err := tui.SelectOne(w.terminal, w.title+" - Header", "Does the first logical record contain column names?", []tui.Choice{
		{Label: "Yes - read header names", Description: "The next screen will show actual header names for Space-key selection."},
		{Label: "No - use column indexes", Description: "Columns will be displayed as column_0, column_1, and so on."},
	}, headerInitial)
	if err != nil {
		return err
	}

	w.cfg.LeftPath = left
	w.cfg.RightPath = right
	w.cfg.OutputPath = output
	w.cfg.HasHeader = headerChoice == 0
	if w.cfg.HasHeader {
		alignInitial := 0
		if !w.cfg.AlignColumnsByName {
			alignInitial = 1
		}
		alignChoice, err := tui.SelectOne(w.terminal, w.title+" - Header alignment", "How should columns in the right file be aligned?", []tui.Choice{
			{Label: "By header name (recommended)", Description: "Right-side columns may appear in a different order."},
			{Label: "By position", Description: "Both header rows must be identical and in the same order."},
		}, alignInitial)
		if err != nil {
			return err
		}
		w.cfg.AlignColumnsByName = alignChoice == 0
	} else {
		w.cfg.AlignColumnsByName = false
	}
	return nil
}

func (w *wizard) inspectWithRecovery() (engine.InputInspection, error) {
	for {
		inspection, err := engine.InspectInputs(w.cfg)
		if err == nil {
			return inspection, nil
		}
		choice, selectErr := tui.SelectOne(w.terminal, w.title+" - Could not read the headers", err.Error(), []tui.Choice{
			{Label: "Edit file paths and header settings"},
			{Label: "Edit CSV/TSV parsing settings"},
			{Label: "Cancel"},
		}, 0)
		if selectErr != nil {
			return engine.InputInspection{}, selectErr
		}
		switch choice {
		case 0:
			if err := w.editFiles(); err != nil {
				return engine.InputInspection{}, err
			}
		case 1:
			if err := w.editParsing(); err != nil {
				return engine.InputInspection{}, err
			}
		case 2:
			return engine.InputInspection{}, ErrCancelled
		}
	}
}

func (w *wizard) editKeys(inspection engine.InputInspection) error {
	initialMode := currentKeyMode(w.cfg)
	modeIndex, err := tui.SelectOne(w.terminal, w.title+" - Key mode",
		fmt.Sprintf("Read %d columns from the input headers.", inspection.ColumnCount), []tui.Choice{
			{Label: "Use every column as the key", Description: "Exact matching rows are equal. Any changed value becomes left-only/right-only."},
			{Label: "Select key columns", Description: "Rows with the same key but changed non-key values are emitted as CHANGED pairs."},
			{Label: "Select columns to exclude from the key", Description: "All columns except the selected volatile columns form the key."},
		}, int(initialMode))
	if err != nil {
		return err
	}
	mode := keyMode(modeIndex)
	if mode == keyModeAll {
		clearKeySelection(&w.cfg)
		w.cfg.IndexBase = 0
		return nil
	}

	initial := initialColumnSelection(w.cfg, inspection.Header, mode)
	options := tui.MultiSelectOptions{
		Initial:        initial,
		IndexBase:      0,
		AdditionalHelp: "Indexes shown here are 0-based. Header names are stored when available.",
	}
	message := "Select one or more columns that uniquely identify a logical row."
	if mode == keyModeInclude {
		options.MinSelected = 1
	} else {
		message = "Select volatile columns to remove from the key. They are still compared and written to the diff."
		options.MaxSelected = maxInt(0, inspection.ColumnCount-1)
		options.MaxSelectedSet = true
	}
	selected, err := tui.MultiSelect(w.terminal, w.title+" - Header selection", message, inspection.Header, options)
	if err != nil {
		return err
	}
	applyColumnSelection(&w.cfg, inspection.Header, selected, mode)
	return nil
}

func (w *wizard) editParsing() error {
	var err error
	w.cfg.LeftFormat, err = chooseString(w.terminal, w.title+" - Left format", "Left input format", w.cfg.LeftFormat, []string{"auto", "csv", "tsv"}, []string{
		"Detect from the extension or first record.", "Treat the input as comma-separated.", "Treat the input as tab-separated.",
	})
	if err != nil {
		return err
	}
	w.cfg.RightFormat, err = chooseString(w.terminal, w.title+" - Right format", "Right input format", w.cfg.RightFormat, []string{"auto", "csv", "tsv"}, []string{
		"Detect from the extension or first record.", "Treat the input as comma-separated.", "Treat the input as tab-separated.",
	})
	if err != nil {
		return err
	}
	w.cfg.LeftParser, err = chooseParser(w.terminal, w.title+" - Left parser", w.cfg.LeftParser)
	if err != nil {
		return err
	}
	w.cfg.RightParser, err = chooseParser(w.terminal, w.title+" - Right parser", w.cfg.RightParser)
	if err != nil {
		return err
	}
	w.cfg.LeftDelimiter, err = tui.PromptLine(w.terminal, w.title+" - Left delimiter", "Delimiter override", w.cfg.LeftDelimiter,
		"Leave empty for automatic format delimiter. Accepted: comma, tab, \\t, pipe, or one ASCII character.", validateDelimiterText)
	if err != nil {
		return err
	}
	w.cfg.RightDelimiter, err = tui.PromptLine(w.terminal, w.title+" - Right delimiter", "Delimiter override", w.cfg.RightDelimiter,
		"Leave empty for automatic format delimiter. Accepted: comma, tab, \\t, pipe, or one ASCII character.", validateDelimiterText)
	if err != nil {
		return err
	}
	w.cfg.LazyQuotes, err = tui.Confirm(w.terminal, w.title+" - CSV compatibility", "Allow malformed quotes in the RFC 4180 parser?", w.cfg.LazyQuotes)
	if err != nil {
		return err
	}
	w.cfg.TrimLeadingSpace, err = tui.Confirm(w.terminal, w.title+" - CSV compatibility", "Trim leading spaces in RFC 4180 fields?", w.cfg.TrimLeadingSpace)
	return err
}

func (w *wizard) editPerformance() error {
	for {
		temp := w.cfg.TempDir
		if temp == "" {
			temp = "(system temporary directory)"
		}
		choice, err := tui.SelectOne(w.terminal, w.title+" - Performance", "Choose a setting to edit.", []tui.Choice{
			{Label: "Memory limit: " + w.cfg.MemoryText},
			{Label: "Temporary directory: " + temp, Description: "For billions of rows, use a fast local NVMe drive with ample free space."},
			{Label: fmt.Sprintf("Hash partitions: %d", w.cfg.Partitions)},
			{Label: fmt.Sprintf("Parallel input readers: %d", w.cfg.ParseWorkers)},
			{Label: fmt.Sprintf("Parallel comparison workers: %d", w.cfg.Workers)},
			{Label: fmt.Sprintf("Merge fan-in: %d", w.cfg.MergeFanIn)},
			{Label: "Partition buffer: " + w.cfg.PartitionBufferText},
			{Label: "Maximum record size: " + w.cfg.MaxRecordText},
			{Label: fmt.Sprintf("Keep temporary files: %t", w.cfg.KeepTemp)},
			{Label: fmt.Sprintf("Show progress: %t", w.cfg.Progress)},
			{Label: fmt.Sprintf("Write output header: %t", w.cfg.OutputHeader)},
			{Label: "Back to review"},
		}, 0)
		if err != nil {
			return err
		}
		switch choice {
		case 0:
			value, err := promptByteSize(w.terminal, w.title+" - Memory", "Total sorting memory", w.cfg.MemoryText, 1)
			if err != nil {
				return err
			}
			w.cfg.MemoryText = value
		case 1:
			value, err := tui.PromptLine(w.terminal, w.title+" - Temporary storage", "Temporary directory (empty uses the OS default)", w.cfg.TempDir,
				"Use a local NVMe path. The directory must already exist.", validateOptionalDirectory)
			if err != nil {
				return err
			}
			w.cfg.TempDir = normalizePathInput(value)
		case 2:
			value, err := promptInt(w.terminal, w.title+" - Partitions", "Power-of-two partition count", w.cfg.Partitions, func(v int) error {
				if v < 2 || v > 1024 || v&(v-1) != 0 {
					return fmt.Errorf("must be a power of two from 2 to 1024")
				}
				return nil
			})
			if err != nil {
				return err
			}
			w.cfg.Partitions = value
		case 3:
			value, err := promptPositiveInt(w.terminal, w.title+" - Input readers", "Parallel input readers", w.cfg.ParseWorkers)
			if err != nil {
				return err
			}
			w.cfg.ParseWorkers = value
		case 4:
			value, err := promptPositiveInt(w.terminal, w.title+" - Workers", "Parallel comparison workers", w.cfg.Workers)
			if err != nil {
				return err
			}
			w.cfg.Workers = value
		case 5:
			value, err := promptInt(w.terminal, w.title+" - Merge", "Maximum sorted runs merged at once", w.cfg.MergeFanIn, func(v int) error {
				if v < 2 || v > 256 {
					return fmt.Errorf("must be from 2 to 256")
				}
				return nil
			})
			if err != nil {
				return err
			}
			w.cfg.MergeFanIn = value
		case 6:
			value, err := promptByteSize(w.terminal, w.title+" - Buffer", "Buffer per partition file", w.cfg.PartitionBufferText, 4*1024)
			if err != nil {
				return err
			}
			bytes, _ := engine.ParseByteSize(value)
			if bytes > 16*1024*1024 {
				if msgErr := tui.ShowMessage(w.terminal, w.title+" - Invalid buffer", []string{"Partition buffer must not exceed 16MiB."}); msgErr != nil {
					return msgErr
				}
				continue
			}
			w.cfg.PartitionBufferText = value
		case 7:
			value, err := promptByteSize(w.terminal, w.title+" - Record size", "Maximum encoded key plus row size", w.cfg.MaxRecordText, 1024)
			if err != nil {
				return err
			}
			w.cfg.MaxRecordText = value
		case 8:
			w.cfg.KeepTemp, err = tui.Confirm(w.terminal, w.title+" - Temporary data", "Keep partition and sort files after completion?", w.cfg.KeepTemp)
			if err != nil {
				return err
			}
		case 9:
			w.cfg.Progress, err = tui.Confirm(w.terminal, w.title+" - Progress", "Print periodic progress while processing?", w.cfg.Progress)
			if err != nil {
				return err
			}
		case 10:
			w.cfg.OutputHeader, err = tui.Confirm(w.terminal, w.title+" - Output", "Write the _diff/_side/header row?", w.cfg.OutputHeader)
			if err != nil {
				return err
			}
		case 11:
			return nil
		}
	}
}

func (w *wizard) reviewText(inspection engine.InputInspection) string {
	keyDescription, keyColumns := describeKeys(w.cfg, inspection.Header)
	temp := w.cfg.TempDir
	if temp == "" {
		temp = "OS default"
	}
	alignment := "by position"
	if w.cfg.HasHeader && w.cfg.AlignColumnsByName {
		alignment = "by header name"
	}
	if inspection.ColumnsReordered {
		alignment += " (right order differs and will be normalized)"
	}
	lines := []string{
		"Left:   " + w.cfg.LeftPath,
		fmt.Sprintf("        %s, %s parser", inspection.LeftFormat, inspection.LeftParser),
		"Right:  " + w.cfg.RightPath,
		fmt.Sprintf("        %s, %s parser", inspection.RightFormat, inspection.RightParser),
		"Output: " + w.cfg.OutputPath,
		fmt.Sprintf("Schema: %d columns, %s", inspection.ColumnCount, alignment),
		"Keys:   " + keyDescription,
	}
	if keyColumns != "" {
		lines = append(lines, "        "+keyColumns)
	}
	lines = append(lines,
		fmt.Sprintf("Scale:  memory=%s, partitions=%d, readers=%d, workers=%d", w.cfg.MemoryText, w.cfg.Partitions, w.cfg.ParseWorkers, w.cfg.Workers),
		"Temp:   "+temp,
	)
	return strings.Join(lines, "\n")
}

func chooseString(t tui.Terminal, title, message, current string, values, descriptions []string) (string, error) {
	choices := make([]tui.Choice, len(values))
	initial := 0
	for i, value := range values {
		description := ""
		if i < len(descriptions) {
			description = descriptions[i]
		}
		choices[i] = tui.Choice{Label: value, Description: description}
		if value == current {
			initial = i
		}
	}
	index, err := tui.SelectOne(t, title, message, choices, initial)
	if err != nil {
		return current, err
	}
	return values[index], nil
}

func chooseParser(t tui.Terminal, title, current string) (string, error) {
	return chooseString(t, title, "Parser mode", current, []string{"auto", "rfc4180", "simple"}, []string{
		"TSV uses the fast parser; CSV uses the safe RFC 4180 parser.",
		"Supports quoted delimiters, escaped quotes, and embedded newlines.",
		"Fast parallel parser. Use only when fields never contain quotes, delimiters, or embedded newlines.",
	})
}

func promptInt(t tui.Terminal, title, label string, current int, validate func(int) error) (int, error) {
	value, err := tui.PromptLine(t, title, label, strconv.Itoa(current), "Enter a base-10 integer.", func(text string) error {
		number, err := strconv.Atoi(strings.TrimSpace(text))
		if err != nil {
			return fmt.Errorf("invalid integer")
		}
		return validate(number)
	})
	if err != nil {
		return current, err
	}
	return strconv.Atoi(strings.TrimSpace(value))
}

func promptPositiveInt(t tui.Terminal, title, label string, current int) (int, error) {
	return promptInt(t, title, label, current, func(value int) error {
		if value < 1 || value > 4096 {
			return fmt.Errorf("must be from 1 to 4096")
		}
		return nil
	})
}

func promptByteSize(t tui.Terminal, title, label, current string, minimum int64) (string, error) {
	return tui.PromptLine(t, title, label, current, "Examples: 512MiB, 8GiB, 32GB.", func(text string) error {
		value, err := engine.ParseByteSize(text)
		if err != nil {
			return err
		}
		if value < minimum {
			return fmt.Errorf("must be at least %d bytes", minimum)
		}
		return nil
	})
}

func currentKeyMode(cfg engine.Config) keyMode {
	if len(cfg.KeyNames)+len(cfg.KeyIndexes) > 0 {
		return keyModeInclude
	}
	if len(cfg.ExcludeKeyNames)+len(cfg.ExcludeKeyIndexes) > 0 {
		return keyModeExclude
	}
	return keyModeAll
}

func initialColumnSelection(cfg engine.Config, header []string, mode keyMode) []bool {
	selected := make([]bool, len(header))
	if currentKeyMode(cfg) != mode {
		return selected
	}
	nameToIndex := make(map[string]int, len(header))
	for i, name := range header {
		nameToIndex[name] = i
	}
	var names []string
	var indexes []int
	if mode == keyModeInclude {
		names, indexes = cfg.KeyNames, cfg.KeyIndexes
	} else {
		names, indexes = cfg.ExcludeKeyNames, cfg.ExcludeKeyIndexes
	}
	for _, name := range names {
		if index, ok := nameToIndex[name]; ok {
			selected[index] = true
		}
	}
	for _, rawIndex := range indexes {
		index := rawIndex - cfg.IndexBase
		if index >= 0 && index < len(selected) {
			selected[index] = true
		}
	}
	return selected
}

func applyColumnSelection(cfg *engine.Config, header []string, selected []bool, mode keyMode) {
	clearKeySelection(cfg)
	cfg.IndexBase = 0
	for index, value := range selected {
		if !value {
			continue
		}
		name := ""
		if index < len(header) {
			name = header[index]
		}
		if cfg.HasHeader && name != "" {
			if mode == keyModeInclude {
				cfg.KeyNames = append(cfg.KeyNames, name)
			} else {
				cfg.ExcludeKeyNames = append(cfg.ExcludeKeyNames, name)
			}
		} else if mode == keyModeInclude {
			cfg.KeyIndexes = append(cfg.KeyIndexes, index)
		} else {
			cfg.ExcludeKeyIndexes = append(cfg.ExcludeKeyIndexes, index)
		}
	}
}

func clearKeySelection(cfg *engine.Config) {
	cfg.KeyNames = nil
	cfg.KeyIndexes = nil
	cfg.ExcludeKeyNames = nil
	cfg.ExcludeKeyIndexes = nil
}

func describeKeys(cfg engine.Config, header []string) (string, string) {
	mode := currentKeyMode(cfg)
	if mode == keyModeAll {
		return "all columns", ""
	}
	selected := initialColumnSelection(cfg, header, mode)
	names := make([]string, 0)
	for i, value := range selected {
		if value {
			name := header[i]
			if name == "" {
				name = fmt.Sprintf("column_%d", i)
			}
			names = append(names, name)
		}
	}
	prefix := "included"
	if mode == keyModeExclude {
		prefix = "excluded"
	}
	return fmt.Sprintf("%s %d column(s)", prefix, len(names)), summarizeNames(names, 8)
}

func summarizeNames(names []string, limit int) string {
	if len(names) == 0 {
		return ""
	}
	shown := names
	if len(shown) > limit {
		shown = shown[:limit]
	}
	display := make([]string, len(shown))
	for i, name := range shown {
		display[i] = tui.InlineText(name)
	}
	text := strings.Join(display, ", ")
	if len(names) > limit {
		text += fmt.Sprintf(" ... (+%d more)", len(names)-limit)
	}
	return text
}

func normalizePathInput(value string) string {
	value = strings.TrimSpace(value)
	for len(value) >= 2 {
		first, last := value[0], value[len(value)-1]
		if (first == '"' && last == '"') || (first == '\'' && last == '\'') {
			value = strings.TrimSpace(value[1 : len(value)-1])
			continue
		}
		break
	}
	if value == "" {
		return ""
	}
	return filepath.Clean(value)
}

func validateInputPath(value string) error {
	path := normalizePathInput(value)
	if path == "" {
		return fmt.Errorf("path is required")
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("path is a directory")
	}
	return nil
}

func validateOutputPath(value, left, right string) error {
	path := normalizePathInput(value)
	if path == "" {
		return fmt.Errorf("path is required")
	}
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		return fmt.Errorf("output path is a directory")
	}
	outputAbs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	for _, input := range []string{left, right} {
		inputAbs, err := filepath.Abs(normalizePathInput(input))
		if err == nil && samePathText(outputAbs, inputAbs) {
			return fmt.Errorf("output must differ from both inputs")
		}
	}
	return nil
}

func validateOptionalDirectory(value string) error {
	path := normalizePathInput(value)
	if path == "" {
		return nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("path is not a directory")
	}
	return nil
}

func validateDelimiterText(value string) error {
	if value == "" {
		return nil
	}
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "comma", "tab", `\t`, "pipe":
		return nil
	}
	if !utf8.ValidString(value) || len(value) != 1 || value[0] >= 0x80 {
		return fmt.Errorf("use comma, tab, \\t, pipe, or one ASCII character")
	}
	if value[0] == '\r' || value[0] == '\n' || value[0] == 0 || value[0] == '"' {
		return fmt.Errorf("unsupported delimiter")
	}
	return nil
}

func defaultOutputPath(left string) string {
	directory := filepath.Dir(left)
	if directory == "" {
		directory = "."
	}
	return filepath.Join(directory, "diff.tsv")
}

func samePathText(a, b string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
	}
	return filepath.Clean(a) == filepath.Clean(b)
}

func cloneConfig(cfg engine.Config) engine.Config {
	cfg.KeyNames = append([]string(nil), cfg.KeyNames...)
	cfg.KeyIndexes = append([]int(nil), cfg.KeyIndexes...)
	cfg.ExcludeKeyNames = append([]string(nil), cfg.ExcludeKeyNames...)
	cfg.ExcludeKeyIndexes = append([]int(nil), cfg.ExcludeKeyIndexes...)
	return cfg
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func IsCancelled(err error) bool {
	return errors.Is(err, ErrCancelled)
}

func IsInterrupted(err error) bool {
	return errors.Is(err, ErrInterrupted)
}
