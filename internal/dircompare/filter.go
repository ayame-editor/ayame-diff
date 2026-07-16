package dircompare

import (
	"fmt"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Filter is an immutable, parsed folder-filter expression.
type Filter struct {
	expression string
	root       filterNode
}

// Expression returns the source expression used to build f.
func (f *Filter) Expression() string {
	if f == nil {
		return ""
	}
	return f.expression
}

// Match reports whether one path and its metadata satisfy the filter.
func (f *Filter) Match(name string, size int64, modTime time.Time) bool {
	return f == nil || f.root == nil || f.root.match(filterValue{path: name, size: size, modTime: modTime})
}

// ParseFilter parses a small boolean expression language. Supported fields are
// size, name, path, ext, and mtime. String fields accept ==, !=, =~, and !~;
// size and mtime also accept <, <=, >, and >=.
func ParseFilter(expression string) (*Filter, error) {
	expression = strings.TrimSpace(expression)
	if expression == "" {
		return nil, nil
	}
	tokens, err := scanFilter(expression)
	if err != nil {
		return nil, err
	}
	p := filterParser{tokens: tokens}
	root, err := p.parseOr()
	if err != nil {
		return nil, err
	}
	if p.more() {
		return nil, fmt.Errorf("filter: unexpected token %q", p.peek())
	}
	return &Filter{expression: expression, root: root}, nil
}

type filterValue struct {
	path    string
	size    int64
	modTime time.Time
}

type filterNode interface{ match(filterValue) bool }
type filterAnd struct{ left, right filterNode }
type filterOr struct{ left, right filterNode }
type filterNot struct{ child filterNode }

func (n filterAnd) match(v filterValue) bool { return n.left.match(v) && n.right.match(v) }
func (n filterOr) match(v filterValue) bool  { return n.left.match(v) || n.right.match(v) }
func (n filterNot) match(v filterValue) bool { return !n.child.match(v) }

type filterPredicate struct {
	field string
	op    string
	text  string
	num   int64
	time  time.Time
	re    *regexp.Regexp
}

func (p filterPredicate) match(v filterValue) bool {
	switch p.field {
	case "size":
		return compareInt(v.size, p.num, p.op)
	case "mtime":
		if v.modTime.IsZero() {
			return false
		}
		return compareInt(v.modTime.UnixNano(), p.time.UnixNano(), p.op)
	default:
		value := v.path
		if p.field == "name" {
			value = path.Base(v.path)
		} else if p.field == "ext" {
			value = path.Ext(v.path)
		}
		switch p.op {
		case "==":
			return value == p.text
		case "!=":
			return value != p.text
		case "=~":
			return p.re.MatchString(value)
		case "!~":
			return !p.re.MatchString(value)
		}
	}
	return false
}

func compareInt(left, right int64, op string) bool {
	switch op {
	case "<":
		return left < right
	case "<=":
		return left <= right
	case "==":
		return left == right
	case "!=":
		return left != right
	case ">=":
		return left >= right
	case ">":
		return left > right
	default:
		return false
	}
}

type filterParser struct {
	tokens []string
	index  int
}

func (p *filterParser) more() bool { return p.index < len(p.tokens) }
func (p *filterParser) peek() string {
	if !p.more() {
		return ""
	}
	return p.tokens[p.index]
}
func (p *filterParser) take() string { token := p.peek(); p.index++; return token }

func (p *filterParser) parseOr() (filterNode, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	for strings.EqualFold(p.peek(), "or") {
		p.take()
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		left = filterOr{left, right}
	}
	return left, nil
}

func (p *filterParser) parseAnd() (filterNode, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	for strings.EqualFold(p.peek(), "and") {
		p.take()
		right, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		left = filterAnd{left, right}
	}
	return left, nil
}

func (p *filterParser) parseUnary() (filterNode, error) {
	if strings.EqualFold(p.peek(), "not") {
		p.take()
		child, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return filterNot{child}, nil
	}
	if p.peek() == "(" {
		p.take()
		node, err := p.parseOr()
		if err != nil {
			return nil, err
		}
		if p.peek() != ")" {
			return nil, fmt.Errorf("filter: missing closing parenthesis")
		}
		p.take()
		return node, nil
	}
	return p.parsePredicate()
}

func (p *filterParser) parsePredicate() (filterNode, error) {
	if p.index+2 >= len(p.tokens) {
		return nil, fmt.Errorf("filter: expected FIELD OP VALUE")
	}
	field := strings.ToLower(p.take())
	op, value := p.take(), p.take()
	if !contains([]string{"size", "name", "path", "ext", "mtime"}, field) {
		return nil, fmt.Errorf("filter: unsupported field %q", field)
	}
	predicate := filterPredicate{field: field, op: op, text: value}
	if field == "size" {
		if !contains([]string{"<", "<=", "==", "!=", ">=", ">"}, op) {
			return nil, fmt.Errorf("filter: size does not support %q", op)
		}
		num, err := parseFilterSize(value)
		if err != nil {
			return nil, fmt.Errorf("filter: size %q: %w", value, err)
		}
		predicate.num = num
	} else if field == "mtime" {
		if !contains([]string{"<", "<=", "==", "!=", ">=", ">"}, op) {
			return nil, fmt.Errorf("filter: mtime does not support %q", op)
		}
		parsed, err := time.Parse(time.RFC3339, value)
		if err != nil {
			parsed, err = time.Parse("2006-01-02", value)
		}
		if err != nil {
			return nil, fmt.Errorf("filter: mtime must be RFC3339 or YYYY-MM-DD")
		}
		predicate.time = parsed
	} else {
		if !contains([]string{"==", "!=", "=~", "!~"}, op) {
			return nil, fmt.Errorf("filter: %s does not support %q", field, op)
		}
		if op == "=~" || op == "!~" {
			compiled, err := regexp.Compile(value)
			if err != nil {
				return nil, fmt.Errorf("filter: invalid regular expression: %w", err)
			}
			predicate.re = compiled
		}
	}
	return predicate, nil
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func parseFilterSize(value string) (int64, error) {
	match := regexp.MustCompile(`(?i)^(\d+(?:\.\d+)?)([kmgt]?i?b)?$`).FindStringSubmatch(value)
	if match == nil {
		return 0, fmt.Errorf("invalid byte size")
	}
	number, err := strconv.ParseFloat(match[1], 64)
	if err != nil {
		return 0, err
	}
	multipliers := map[string]float64{"": 1, "b": 1, "kb": 1000, "mb": 1000 * 1000, "gb": 1000 * 1000 * 1000, "tb": 1000 * 1000 * 1000 * 1000, "kib": 1 << 10, "mib": 1 << 20, "gib": 1 << 30, "tib": 1 << 40}
	result := number * multipliers[strings.ToLower(match[2])]
	if result < 0 || result > float64(^uint64(0)>>1) {
		return 0, fmt.Errorf("byte size out of range")
	}
	return int64(result), nil
}

func scanFilter(expression string) ([]string, error) {
	var tokens []string
	for i := 0; i < len(expression); {
		if expression[i] == ' ' || expression[i] == '\t' || expression[i] == '\r' || expression[i] == '\n' {
			i++
			continue
		}
		if expression[i] == '(' || expression[i] == ')' {
			tokens = append(tokens, expression[i:i+1])
			i++
			continue
		}
		if expression[i] == '\'' || expression[i] == '"' {
			quote := expression[i]
			i++
			var value strings.Builder
			closed := false
			for i < len(expression) {
				if expression[i] == quote {
					i++
					closed = true
					break
				}
				if expression[i] == '\\' && i+1 < len(expression) {
					next := expression[i+1]
					if next == quote || next == '\\' {
						value.WriteByte(next)
					} else {
						value.WriteByte('\\')
						value.WriteByte(next)
					}
					i += 2
					continue
				}
				value.WriteByte(expression[i])
				i++
			}
			if !closed {
				return nil, fmt.Errorf("filter: unterminated quoted value")
			}
			tokens = append(tokens, value.String())
			continue
		}
		matchedOperator := ""
		for _, operator := range []string{"<=", ">=", "==", "!=", "=~", "!~", "<", ">"} {
			if strings.HasPrefix(expression[i:], operator) {
				matchedOperator = operator
				break
			}
		}
		if matchedOperator != "" {
			tokens = append(tokens, matchedOperator)
			i += len(matchedOperator)
			continue
		}
		start := i
		for i < len(expression) && !strings.ContainsRune(" \t\r\n()<>!=~", rune(expression[i])) {
			i++
		}
		if start == i {
			return nil, fmt.Errorf("filter: unexpected character %q", expression[i])
		}
		tokens = append(tokens, expression[start:i])
	}
	return tokens, nil
}
