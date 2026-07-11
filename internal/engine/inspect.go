package engine

import (
	"fmt"
	"reflect"
)

// InputInspection contains schema data for interactive clients. InspectInputs
// reads only the first logical record of each input; it never scans data rows.
type InputInspection struct {
	Header           []string `json:"header"`
	LeftHeader       []string `json:"left_header"`
	RightHeader      []string `json:"right_header"`
	ColumnCount      int      `json:"column_count"`
	LeftFormat       string   `json:"left_format"`
	RightFormat      string   `json:"right_format"`
	LeftParser       string   `json:"left_parser"`
	RightParser      string   `json:"right_parser"`
	ColumnsReordered bool     `json:"columns_reordered"`
}

func InspectInputs(cfg Config) (InputInspection, error) {
	if cfg.LeftPath == "" || cfg.RightPath == "" {
		return InputInspection{}, fmt.Errorf("left and right input paths are required")
	}
	leftSpec, err := resolveInputSpec(cfg.LeftPath, defaultString(cfg.LeftFormat, "auto"), cfg.LeftDelimiter, defaultString(cfg.LeftParser, "auto"), "left")
	if err != nil {
		return InputInspection{}, err
	}
	rightSpec, err := resolveInputSpec(cfg.RightPath, defaultString(cfg.RightFormat, "auto"), cfg.RightDelimiter, defaultString(cfg.RightParser, "auto"), "right")
	if err != nil {
		return InputInspection{}, err
	}
	leftInfo, err := inspectInput(leftSpec, cfg.HasHeader, cfg.LazyQuotes, cfg.TrimLeadingSpace)
	if err != nil {
		return InputInspection{}, err
	}
	rightInfo, err := inspectInput(rightSpec, cfg.HasHeader, cfg.LazyQuotes, cfg.TrimLeadingSpace)
	if err != nil {
		return InputInspection{}, err
	}
	schemaCfg := cfg
	schemaCfg.KeyNames, schemaCfg.KeyIndexes, schemaCfg.ExcludeKeyNames, schemaCfg.ExcludeKeyIndexes = nil, nil, nil, nil
	schemaCfg.IgnoreColumnNames, schemaCfg.IgnoreColumnIndexes, schemaCfg.ColumnTolerances = nil, nil, nil
	schemaCfg.ToleranceSet = false
	resolved, err := buildSchema(leftInfo, rightInfo, schemaCfg)
	if err != nil {
		return InputInspection{}, err
	}
	return InputInspection{
		Header: append([]string(nil), resolved.Header...), LeftHeader: append([]string(nil), leftInfo.Header...),
		RightHeader: append([]string(nil), rightInfo.Header...), ColumnCount: resolved.ColumnCount,
		LeftFormat: inputFormatLabel(leftSpec), RightFormat: inputFormatLabel(rightSpec),
		LeftParser: parserLabel(leftSpec.Parser), RightParser: parserLabel(rightSpec.Parser),
		ColumnsReordered: cfg.HasHeader && !reflect.DeepEqual(leftInfo.Header, rightInfo.Header),
	}, nil
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func inputFormatLabel(spec inputSpec) string {
	label := "CSV"
	if spec.Delimiter == '\t' {
		label = "TSV"
	} else if spec.Delimiter != ',' {
		label = fmt.Sprintf("delimited %q", rune(spec.Delimiter))
	}
	if spec.Compressed {
		label += " (gzip)"
	}
	return label
}

func parserLabel(kind parserKind) string {
	if kind == parserRFC4180 {
		return "RFC 4180"
	}
	return "simple"
}
