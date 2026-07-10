package engine

import (
	"fmt"
	"reflect"
)

// InputInspection contains the schema information needed by the interactive
// setup wizard. InspectInputs reads only the first logical record of each
// input; it does not scan the data files.
type InputInspection struct {
	Header           []string
	LeftHeader       []string
	RightHeader      []string
	ColumnCount      int
	LeftFormat       string
	RightFormat      string
	LeftParser       string
	RightParser      string
	ColumnsReordered bool
}

func InspectInputs(cfg Config) (InputInspection, error) {
	if cfg.LeftPath == "" || cfg.RightPath == "" {
		return InputInspection{}, fmt.Errorf("left and right input paths are required")
	}
	leftSpec, err := resolveInputSpec(cfg.LeftPath, cfg.LeftFormat, cfg.LeftDelimiter, cfg.LeftParser, "left")
	if err != nil {
		return InputInspection{}, err
	}
	rightSpec, err := resolveInputSpec(cfg.RightPath, cfg.RightFormat, cfg.RightDelimiter, cfg.RightParser, "right")
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
	schemaCfg.KeyNames = nil
	schemaCfg.KeyIndexes = nil
	schemaCfg.ExcludeKeyNames = nil
	schemaCfg.ExcludeKeyIndexes = nil
	resolved, err := buildSchema(leftInfo, rightInfo, schemaCfg)
	if err != nil {
		return InputInspection{}, err
	}

	return InputInspection{
		Header:           append([]string(nil), resolved.Header...),
		LeftHeader:       append([]string(nil), leftInfo.Header...),
		RightHeader:      append([]string(nil), rightInfo.Header...),
		ColumnCount:      resolved.ColumnCount,
		LeftFormat:       inputFormatLabel(leftSpec),
		RightFormat:      inputFormatLabel(rightSpec),
		LeftParser:       parserLabel(leftSpec.Parser),
		RightParser:      parserLabel(rightSpec.Parser),
		ColumnsReordered: cfg.HasHeader && !reflect.DeepEqual(leftInfo.Header, rightInfo.Header),
	}, nil
}

func inputFormatLabel(spec inputSpec) string {
	switch spec.Delimiter {
	case ',':
		if spec.Compressed {
			return "CSV (gzip)"
		}
		return "CSV"
	case '\t':
		if spec.Compressed {
			return "TSV (gzip)"
		}
		return "TSV"
	default:
		if spec.Compressed {
			return fmt.Sprintf("delimited %q (gzip)", rune(spec.Delimiter))
		}
		return fmt.Sprintf("delimited %q", rune(spec.Delimiter))
	}
}

func parserLabel(kind parserKind) string {
	if kind == parserRFC4180 {
		return "RFC 4180"
	}
	return "simple"
}
