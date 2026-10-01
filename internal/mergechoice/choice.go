// Package mergechoice parses the ordered, comma-joined side lists the merge
// surfaces send. A hunk may adopt more than one contribution ("both"), so a
// choice value such as "left,right" names every adopted side; the canonical
// order passed by the caller fixes the order the engine concatenates them in,
// independent of which toggle the user flipped first (#271).
package mergechoice

import (
	"fmt"
	"strings"
)

// Validate reports whether value is a non-empty, duplicate-free list of sides
// drawn from order. It is used to reject malformed client choices before the
// engine sees them.
func Validate(value string, order ...string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("empty merge choice")
	}
	allowed := make(map[string]bool, len(order))
	for _, side := range order {
		allowed[side] = true
	}
	seen := make(map[string]bool)
	for _, token := range strings.Split(value, ",") {
		token = strings.TrimSpace(token)
		if !allowed[token] {
			return fmt.Errorf("unknown merge side %q", token)
		}
		if seen[token] {
			return fmt.Errorf("duplicate merge side %q", token)
		}
		seen[token] = true
	}
	return nil
}

// Parse returns the adopted sides of value in canonical order. Tokens outside
// order are dropped, so callers should Validate first when the input is
// untrusted. An empty value yields nil.
func Parse(value string, order ...string) []string {
	selected := make(map[string]bool)
	for _, token := range strings.Split(value, ",") {
		token = strings.TrimSpace(token)
		if token != "" {
			selected[token] = true
		}
	}
	var out []string
	for _, side := range order {
		if selected[side] {
			out = append(out, side)
			delete(selected, side)
		}
	}
	return out
}

// Has reports whether side is among the adopted sides encoded in value.
func Has(value, side string) bool {
	for _, token := range strings.Split(value, ",") {
		if strings.TrimSpace(token) == side {
			return true
		}
	}
	return false
}
