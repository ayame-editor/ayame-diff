package engine

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// RowFilter is the declarative, SQL-free row-inclusion condition a GUI filter
// builder compiles to (#129). It is a bounded tree of leaf conditions combined
// with AND/OR and optional negation. The engine compiles it once per comparison
// into a streaming predicate evaluated on the parser's hot path, so filtering
// costs one pass and constant memory — it never collects rows to filter them.
//
// ColumnFilter reuses the same tree shape but its leaves are evaluated against
// column *names*; every header whose name matches becomes an ignored column,
// which is the engine's existing ignore-column mechanism.
type RowFilter struct {
	// Match is "all" (AND, default) or "any" (OR) at this level.
	Match string `json:"match,omitempty"`
	// IgnoreCase folds letter case for every string operation below this node.
	IgnoreCase bool `json:"ignore_case,omitempty"`
	// Not negates the result of this node.
	Not bool `json:"not,omitempty"`
	// Conditions are leaf value tests.
	Conditions []RowCondition `json:"conditions,omitempty"`
	// Groups are nested sub-trees combined with the same Match operator.
	Groups []RowFilter `json:"groups,omitempty"`
}

// RowCondition is one leaf test: Column/Index select the subject and Op names
// the comparison. ByIndex selects a headerless positional column.
type RowCondition struct {
	Column  string `json:"column,omitempty"`
	Index   int    `json:"index,omitempty"`
	ByIndex bool   `json:"by_index,omitempty"`
	Op      string `json:"op"`
	Value   string `json:"value,omitempty"`
	Value2  string `json:"value2,omitempty"`
	Not     bool   `json:"not,omitempty"`
}

// Operator names accepted in RowCondition.Op. They are the closed vocabulary the
// UI offers; anything else is rejected rather than guessed.
const (
	filterOpEq        = "eq"
	filterOpNe        = "ne"
	filterOpContains  = "contains"
	filterOpStarts    = "starts"
	filterOpEnds      = "ends"
	filterOpGt        = "gt"
	filterOpGe        = "ge"
	filterOpLt        = "lt"
	filterOpLe        = "le"
	filterOpBetween   = "between"
	filterOpEmpty     = "empty"
	filterOpNotEmpty  = "not_empty"
	filterOpRegex     = "regex"
	filterMaxDepth    = 8
	filterMaxLeaves   = 512
	filterMaxValueLen = 64 * 1024
)

func filterOpKnown(op string) bool {
	switch op {
	case filterOpEq, filterOpNe, filterOpContains, filterOpStarts, filterOpEnds,
		filterOpGt, filterOpGe, filterOpLt, filterOpLe, filterOpBetween,
		filterOpEmpty, filterOpNotEmpty, filterOpRegex:
		return true
	default:
		return false
	}
}

// compiledFilter is the validated, allocation-free evaluation form. Conditions
// and groups are flattened into slices so a row walk does no map lookups.
type compiledFilter struct {
	matchAll   bool
	negate     bool
	conditions []compiledCondition
	groups     []*compiledFilter
}

type compiledCondition struct {
	index      int // logical column index for row mode, -1 for column-name mode
	op         string
	value      string
	value2     string
	number     float64
	number2    float64
	numeric    bool
	re         *regexp.Regexp
	negate     bool
	ignoreCase bool
}

// compileRowFilter validates a row filter against the header and resolves the
// columns. nameMode compiles the same tree for column-name matching, where the
// leaf selector is intentionally ignored.
func compileRowFilter(header []string, f *RowFilter, nameMode bool, label string) (*compiledFilter, error) {
	if f == nil {
		return nil, nil
	}
	byName := indexHeaders(header)
	leaves := 0
	compiled, err := compileFilterNode(header, byName, f, nameMode, false, label, 0, &leaves)
	if err != nil {
		return nil, err
	}
	return compiled, nil
}

func compileFilterNode(header []string, byName map[string]int, f *RowFilter, nameMode, ignoreCase bool, label string, depth int, leaves *int) (*compiledFilter, error) {
	if depth > filterMaxDepth {
		return nil, fmt.Errorf("%s is nested more than %d levels deep", label, filterMaxDepth)
	}
	matchAll := true
	switch f.Match {
	case "", "all", "and":
	case "any", "or":
		matchAll = false
	default:
		return nil, fmt.Errorf("%s has unknown match mode %q", label, f.Match)
	}
	ignoreCase = ignoreCase || f.IgnoreCase
	result := &compiledFilter{matchAll: matchAll, negate: f.Not}
	for _, condition := range f.Conditions {
		*leaves++
		if *leaves > filterMaxLeaves {
			return nil, fmt.Errorf("%s has more than %d conditions", label, filterMaxLeaves)
		}
		compiled, err := compileFilterCondition(header, byName, condition, nameMode, ignoreCase, label)
		if err != nil {
			return nil, err
		}
		result.conditions = append(result.conditions, compiled)
	}
	for i := range f.Groups {
		group, err := compileFilterNode(header, byName, &f.Groups[i], nameMode, ignoreCase, label, depth+1, leaves)
		if err != nil {
			return nil, err
		}
		result.groups = append(result.groups, group)
	}
	return result, nil
}

func compileFilterCondition(header []string, byName map[string]int, condition RowCondition, nameMode, ignoreCase bool, label string) (compiledCondition, error) {
	if !filterOpKnown(condition.Op) {
		return compiledCondition{}, fmt.Errorf("%s has unknown operator %q", label, condition.Op)
	}
	if len(condition.Value) > filterMaxValueLen || len(condition.Value2) > filterMaxValueLen {
		return compiledCondition{}, fmt.Errorf("%s value is longer than %d bytes", label, filterMaxValueLen)
	}
	compiled := compiledCondition{
		index:      -1,
		op:         condition.Op,
		value:      condition.Value,
		value2:     condition.Value2,
		negate:     condition.Not,
		ignoreCase: ignoreCase,
	}
	if ignoreCase {
		compiled.value = strings.ToLower(compiled.value)
		compiled.value2 = strings.ToLower(compiled.value2)
	}
	if !nameMode {
		switch {
		case condition.ByIndex:
			if condition.Index < 0 || condition.Index >= len(header) {
				return compiledCondition{}, fmt.Errorf("%s column index %d is outside 0..%d", label, condition.Index, len(header)-1)
			}
			compiled.index = condition.Index
		case strings.TrimSpace(condition.Column) != "":
			index, ok := byName[condition.Column]
			if !ok {
				return compiledCondition{}, fmt.Errorf("%s column %q not found in the header", label, condition.Column)
			}
			compiled.index = index
		default:
			return compiledCondition{}, fmt.Errorf("%s condition requires a column", label)
		}
	}
	switch condition.Op {
	case filterOpRegex:
		pattern := condition.Value
		if ignoreCase {
			pattern = "(?i)" + pattern
		}
		re, err := regexp.Compile(pattern)
		if err != nil {
			return compiledCondition{}, fmt.Errorf("%s regular expression %q is invalid: %w", label, condition.Value, err)
		}
		compiled.re = re
	case filterOpGt, filterOpGe, filterOpLt, filterOpLe, filterOpBetween:
		if number, ok := parseFilterNumber(condition.Value); ok {
			compiled.number, compiled.numeric = number, true
		}
		if condition.Op == filterOpBetween {
			if number, ok := parseFilterNumber(condition.Value2); ok {
				compiled.number2 = number
			} else {
				compiled.numeric = false
			}
		}
	}
	return compiled, nil
}

func parseFilterNumber(text string) (float64, bool) {
	value, err := strconv.ParseFloat(strings.TrimSpace(text), 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, false
	}
	return value, true
}

// matchCondition evaluates one leaf against a string value.
func (c *compiledCondition) match(value string) bool {
	result := c.evaluate(value)
	if c.negate {
		return !result
	}
	return result
}

func (c *compiledCondition) evaluate(value string) bool {
	if c.op == filterOpRegex {
		return c.re.MatchString(value)
	}
	if c.ignoreCase {
		value = strings.ToLower(value)
	}
	switch c.op {
	case filterOpEq:
		return value == c.value
	case filterOpNe:
		return value != c.value
	case filterOpContains:
		return strings.Contains(value, c.value)
	case filterOpStarts:
		return strings.HasPrefix(value, c.value)
	case filterOpEnds:
		return strings.HasSuffix(value, c.value)
	case filterOpEmpty:
		return value == ""
	case filterOpNotEmpty:
		return value != ""
	case filterOpGt, filterOpGe, filterOpLt, filterOpLe, filterOpBetween:
		return c.compare(value)
	default:
		return false
	}
}

func (c *compiledCondition) compare(value string) bool {
	if c.numeric {
		number, ok := parseFilterNumber(value)
		if !ok {
			return false
		}
		switch c.op {
		case filterOpGt:
			return number > c.number
		case filterOpGe:
			return number >= c.number
		case filterOpLt:
			return number < c.number
		case filterOpLe:
			return number <= c.number
		case filterOpBetween:
			low, high := c.number, c.number2
			if low > high {
				low, high = high, low
			}
			return number >= low && number <= high
		}
		return false
	}
	// Non-numeric operands compare lexicographically (byte order), which keeps
	// dates and identifiers usable without pretending they are numbers.
	switch c.op {
	case filterOpGt:
		return value > c.value
	case filterOpGe:
		return value >= c.value
	case filterOpLt:
		return value < c.value
	case filterOpLe:
		return value <= c.value
	case filterOpBetween:
		low, high := c.value, c.value2
		if low > high {
			low, high = high, low
		}
		return value >= low && value <= high
	}
	return false
}

func (f *compiledFilter) result(anyMatch bool) bool {
	if f.negate {
		return !anyMatch
	}
	return anyMatch
}

// matchFilterFields evaluates the filter against one parsed row. mapping maps
// logical columns to the record's physical positions.
func matchFilterFields[T fieldBytes](f *compiledFilter, fields []T, mapping []int) bool {
	if f == nil {
		return true
	}
	any := false
	for i := range f.conditions {
		condition := &f.conditions[i]
		value := ""
		if condition.index >= 0 && condition.index < len(mapping) {
			physical := mapping[condition.index]
			if physical >= 0 && physical < len(fields) {
				value = string(fields[physical])
			}
		}
		if condition.match(value) {
			if !f.matchAll {
				any = true
				break
			}
		} else if f.matchAll {
			return f.result(false)
		}
	}
	if f.matchAll {
		for i := range f.groups {
			if !matchFilterFields(f.groups[i], fields, mapping) {
				return f.result(false)
			}
		}
		return f.result(true)
	}
	if !any {
		for i := range f.groups {
			if matchFilterFields(f.groups[i], fields, mapping) {
				any = true
				break
			}
		}
	}
	return f.result(any)
}

// matchFilterName evaluates a column-name filter against one header entry.
func matchFilterName(f *compiledFilter, name string) bool {
	if f == nil {
		return false
	}
	any := false
	for i := range f.conditions {
		if f.conditions[i].match(name) {
			if !f.matchAll {
				any = true
				break
			}
		} else if f.matchAll {
			return f.result(false)
		}
	}
	if f.matchAll {
		for i := range f.groups {
			if !matchFilterName(f.groups[i], name) {
				return f.result(false)
			}
		}
		return f.result(true)
	}
	if !any {
		for i := range f.groups {
			if matchFilterName(f.groups[i], name) {
				any = true
				break
			}
		}
	}
	return f.result(any)
}
