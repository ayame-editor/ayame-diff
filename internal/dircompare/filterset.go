package dircompare

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

// FilterSet is a reusable folder selection. Includes use OR semantics,
// excludes remove matches, and Expression is combined with both.
type FilterSet struct {
	Description string   `json:"description,omitempty"`
	Includes    []string `json:"includes,omitempty"`
	Excludes    []string `json:"excludes,omitempty"`
	Expression  string   `json:"expression,omitempty"`
}

// DirectoryProject is the directory section embedded in an .ayamediff.json
// project. Paths are resolved by the project package; filter documents may omit
// them and carry reusable sets only.
type DirectoryProject struct {
	Old        string   `json:"old,omitempty"`
	New        string   `json:"new,omitempty"`
	Includes   []string `json:"includes,omitempty"`
	Excludes   []string `json:"excludes,omitempty"`
	Filter     string   `json:"filter,omitempty"`
	FilterSets []string `json:"filter_sets,omitempty"`
	CompareBy  string   `json:"compare_by,omitempty"`
	Hidden     bool     `json:"hidden,omitempty"`
	Workers    int      `json:"workers,omitempty"`
}

type filterDocument struct {
	Version   int                  `json:"version"`
	Default   string               `json:"default,omitempty"`
	Filters   map[string]FilterSet `json:"filters,omitempty"`
	Mode      string               `json:"mode,omitempty"`
	Directory *DirectoryProject    `json:"directory,omitempty"`
}

var builtinFilterSets = map[string]FilterSet{
	"vcs": {
		Description: "version-control metadata",
		Excludes:    []string{".git/**", ".hg/**", ".svn/**"},
	},
	"node": {
		Description: "Node.js generated dependencies and output",
		Excludes:    []string{"node_modules/**", ".next/**", "dist/**"},
	},
	"rust": {
		Description: "Rust build output",
		Excludes:    []string{"target/**"},
	},
	"development": {
		Description: "common VCS, dependency, build, and temporary paths",
		Excludes: []string{
			".git/**", ".hg/**", ".svn/**", "node_modules/**", ".next/**",
			"target/**", "dist/**", "build/**", "*.tmp", "*.swp",
		},
	},
}

// BuiltinFilterSetNames returns stable names accepted without a filter file.
func BuiltinFilterSetNames() []string {
	names := make([]string, 0, len(builtinFilterSets))
	for name := range builtinFilterSets {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// ResolveFilterSets loads an optional named-set document and combines selected
// sets. An embedded directory project contributes its own settings and set
// names. With no explicit names, the document's default is selected.
func ResolveFilterSets(filePath string, selected []string) (FilterSet, *DirectoryProject, error) {
	sets := make(map[string]FilterSet, len(builtinFilterSets))
	for name, set := range builtinFilterSets {
		sets[name] = set
	}
	var document filterDocument
	if filePath != "" {
		data, err := os.ReadFile(filePath)
		if err != nil {
			return FilterSet{}, nil, err
		}
		if err := json.Unmarshal(data, &document); err != nil {
			return FilterSet{}, nil, fmt.Errorf("decode filter file: %w", err)
		}
		if document.Version != 0 && document.Version != 1 {
			return FilterSet{}, nil, fmt.Errorf("unsupported filter file version %d", document.Version)
		}
		for name, set := range document.Filters {
			sets[name] = set
		}
	}
	names := append([]string(nil), selected...)
	if len(names) == 0 && document.Default != "" {
		names = append(names, document.Default)
	}
	if document.Directory != nil {
		names = append(names, document.Directory.FilterSets...)
	}
	combined := FilterSet{}
	if document.Directory != nil {
		combined.Includes = append(combined.Includes, document.Directory.Includes...)
		combined.Excludes = append(combined.Excludes, document.Directory.Excludes...)
		combined.Expression = strings.TrimSpace(document.Directory.Filter)
	}
	seen := map[string]bool{}
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" || seen[name] {
			continue
		}
		set, ok := sets[name]
		if !ok {
			return FilterSet{}, document.Directory, fmt.Errorf("unknown filter set %q", name)
		}
		seen[name] = true
		combined.Includes = append(combined.Includes, set.Includes...)
		combined.Excludes = append(combined.Excludes, set.Excludes...)
		combined.Expression = andExpression(combined.Expression, set.Expression)
	}
	return combined, document.Directory, nil
}

func andExpression(left, right string) string {
	left, right = strings.TrimSpace(left), strings.TrimSpace(right)
	if left == "" {
		return right
	}
	if right == "" {
		return left
	}
	return "(" + left + ") and (" + right + ")"
}
