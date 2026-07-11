package engine

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

type comparisonConfig struct {
	enabled       bool
	ignoreCase    bool
	whitespace    string
	filters       []*regexp.Regexp
	ignoreColumns []bool
	tolerances    []float64
	toleranceSet  []bool
	global        float64
	globalSet     bool
}

type preparedComparison struct {
	values    []string
	numbers   []float64
	numericOK []bool
	signature string
}

func buildComparisonConfig(header []string, cfg Config) (comparisonConfig, error) {
	c := comparisonConfig{
		ignoreCase: cfg.IgnoreCase, whitespace: cfg.IgnoreWhitespace,
		ignoreColumns: make([]bool, len(header)), tolerances: make([]float64, len(header)),
		toleranceSet: make([]bool, len(header)), global: cfg.Tolerance, globalSet: cfg.ToleranceSet,
	}
	if c.whitespace == "" {
		c.whitespace = "none"
	}
	for _, pattern := range cfg.LineFilters {
		filter, err := regexp.Compile(pattern)
		if err != nil {
			return comparisonConfig{}, fmt.Errorf("invalid line filter %q: %w", pattern, err)
		}
		c.filters = append(c.filters, filter)
	}
	indexes, err := resolveComparisonColumns(header, cfg.IgnoreColumnNames, cfg.IgnoreColumnIndexes, "ignored")
	if err != nil {
		return comparisonConfig{}, err
	}
	for _, index := range indexes {
		c.ignoreColumns[index] = true
	}
	for _, tolerance := range cfg.ColumnTolerances {
		index := tolerance.Index
		if !tolerance.ByIndex {
			var ok bool
			index, ok = indexHeaders(header)[tolerance.Name]
			if !ok {
				return comparisonConfig{}, fmt.Errorf("tolerance column %q not found in left header", tolerance.Name)
			}
		}
		if index < 0 || index >= len(header) {
			return comparisonConfig{}, fmt.Errorf("tolerance column index %d is outside 0..%d", index, len(header)-1)
		}
		c.tolerances[index], c.toleranceSet[index] = tolerance.Value, true
	}
	c.enabled = c.ignoreCase || c.whitespace != "none" || len(c.filters) > 0 ||
		len(indexes) > 0 || c.globalSet || len(cfg.ColumnTolerances) > 0
	return c, nil
}

func resolveComparisonColumns(header, names []string, indexes []int, label string) ([]int, error) {
	resolved := make([]int, 0, len(names)+len(indexes))
	seen := make(map[int]bool)
	byName := indexHeaders(header)
	for _, name := range names {
		index, ok := byName[name]
		if !ok {
			return nil, fmt.Errorf("%s column %q not found in left header", label, name)
		}
		if !seen[index] {
			seen[index], resolved = true, append(resolved, index)
		}
	}
	for _, index := range indexes {
		if index < 0 || index >= len(header) {
			return nil, fmt.Errorf("%s column index %d is outside 0..%d", label, index, len(header)-1)
		}
		if !seen[index] {
			seen[index], resolved = true, append(resolved, index)
		}
	}
	return resolved, nil
}

func (c comparisonConfig) defaultKeys(keys []int) []int {
	result := keys[:0]
	for _, index := range keys {
		if !c.ignoreColumns[index] && !c.toleranceSet[index] {
			result = append(result, index)
		}
	}
	return result
}

func (c comparisonConfig) validateToleranceKeys(keys []int) error {
	for _, index := range keys {
		if c.toleranceSet[index] {
			return fmt.Errorf("numeric tolerance column %d cannot also be a key column", index)
		}
	}
	return nil
}

func (c comparisonConfig) normalize(value string) string {
	for _, filter := range c.filters {
		value = filter.ReplaceAllString(value, "")
	}
	switch c.whitespace {
	case "all":
		value = strings.Map(func(r rune) rune {
			if unicode.IsSpace(r) {
				return -1
			}
			return r
		}, value)
	case "change":
		value = strings.Join(strings.Fields(value), " ")
	}
	if c.ignoreCase {
		value = strings.ToLower(value)
	}
	return value
}

func (c comparisonConfig) hasTolerance() bool {
	if c.globalSet {
		return true
	}
	for _, set := range c.toleranceSet {
		if set {
			return true
		}
	}
	return false
}

func (c comparisonConfig) prepare(fields []string) preparedComparison {
	p := preparedComparison{
		values: make([]string, len(fields)), numbers: make([]float64, len(fields)), numericOK: make([]bool, len(fields)),
	}
	var signature strings.Builder
	for index, field := range fields {
		if c.ignoreColumns[index] {
			continue
		}
		value := c.normalize(field)
		p.values[index] = value
		signature.WriteString(strconv.Itoa(len(value)))
		signature.WriteByte(':')
		signature.WriteString(value)
		signature.WriteByte(';')
		if c.globalSet || c.toleranceSet[index] {
			number, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
			if err == nil && !math.IsNaN(number) && !math.IsInf(number, 0) {
				p.numbers[index], p.numericOK[index] = number, true
			}
		}
	}
	p.signature = signature.String()
	return p
}

func (c comparisonConfig) equivalentPrepared(left, right preparedComparison) bool {
	for index := range left.values {
		if c.ignoreColumns[index] || left.values[index] == right.values[index] {
			continue
		}
		tolerance, enabled := c.global, c.globalSet
		if c.toleranceSet[index] {
			tolerance, enabled = c.tolerances[index], true
		}
		if !enabled || !left.numericOK[index] || !right.numericOK[index] ||
			math.Abs(left.numbers[index]-right.numbers[index]) > tolerance {
			return false
		}
	}
	return true
}
