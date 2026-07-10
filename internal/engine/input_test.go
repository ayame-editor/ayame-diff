package engine

import (
	"reflect"
	"testing"
)

func TestBuildSchemaAlignsRightHeader(t *testing.T) {
	t.Parallel()
	left := inspectedInput{Header: []string{"id", "region", "value"}, ColumnCount: 3}
	right := inspectedInput{Header: []string{"value", "id", "region"}, ColumnCount: 3}
	cfg := Config{HasHeader: true, AlignColumnsByName: true, KeyNames: []string{"id", "region"}}
	got, err := buildSchema(left, right, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.RightMap, []int{1, 2, 0}) {
		t.Fatalf("right map = %#v", got.RightMap)
	}
	if !reflect.DeepEqual(got.KeyIndexes, []int{0, 1}) {
		t.Fatalf("key indexes = %#v", got.KeyIndexes)
	}
}

func TestBuildSchemaRejectsDuplicateHeader(t *testing.T) {
	t.Parallel()
	left := inspectedInput{Header: []string{"id", "id"}, ColumnCount: 2}
	right := inspectedInput{Header: []string{"id", "id"}, ColumnCount: 2}
	cfg := Config{HasHeader: true, AlignColumnsByName: true, KeyNames: []string{"id"}}
	if _, err := buildSchema(left, right, cfg); err == nil {
		t.Fatal("expected duplicate header error")
	}
}

func TestBuildSchemaDefaultsToAllColumns(t *testing.T) {
	t.Parallel()
	left := inspectedInput{Header: []string{"id", "region", "value"}, ColumnCount: 3}
	right := inspectedInput{Header: []string{"region", "value", "id"}, ColumnCount: 3}
	cfg := Config{HasHeader: true, AlignColumnsByName: true}
	got, err := buildSchema(left, right, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.KeyIndexes, []int{0, 1, 2}) {
		t.Fatalf("key indexes = %#v, want all columns", got.KeyIndexes)
	}
}

func TestBuildSchemaExcludesKeysByNameAndIndex(t *testing.T) {
	t.Parallel()
	left := inspectedInput{Header: []string{"id", "region", "value", "updated_at"}, ColumnCount: 4}
	right := inspectedInput{Header: []string{"id", "region", "value", "updated_at"}, ColumnCount: 4}
	cfg := Config{
		HasHeader:          true,
		AlignColumnsByName: true,
		ExcludeKeyNames:    []string{"updated_at"},
		ExcludeKeyIndexes:  []int{2},
	}
	got, err := buildSchema(left, right, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.KeyIndexes, []int{0, 1}) {
		t.Fatalf("key indexes = %#v, want [0 1]", got.KeyIndexes)
	}
}

func TestBuildSchemaRejectsExcludingEveryColumn(t *testing.T) {
	t.Parallel()
	left := inspectedInput{Header: []string{"id", "value"}, ColumnCount: 2}
	right := inspectedInput{Header: []string{"id", "value"}, ColumnCount: 2}
	cfg := Config{HasHeader: true, AlignColumnsByName: true, ExcludeKeyIndexes: []int{0, 1}}
	if _, err := buildSchema(left, right, cfg); err == nil {
		t.Fatal("expected no-key-columns error")
	}
}
