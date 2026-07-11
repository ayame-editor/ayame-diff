package engine

import (
	"reflect"
	"testing"
)

func TestAlignHeaders(t *testing.T) {
	t.Parallel()
	left, right, err := alignHeaders([]string{"id", "name", "value"}, []string{"value", "id", "name"}, 3, true, true)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(left, []int{0, 1, 2}) || !reflect.DeepEqual(right, []int{1, 2, 0}) {
		t.Fatalf("maps = left %#v right %#v", left, right)
	}
	if _, _, err := alignHeaders([]string{"id"}, []string{"other"}, 1, true, false); err == nil {
		t.Fatal("unaligned different headers succeeded")
	}
}

func TestResolveIncludeAndExcludeKeys(t *testing.T) {
	t.Parallel()
	header := []string{"id", "region", "value", "updated"}
	included, err := resolveIncludeKeys(header, len(header), []string{"region", "id", "region"}, []int{2})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(included, []int{1, 0, 2}) {
		t.Fatalf("included = %#v", included)
	}
	excluded, err := resolveExcludeKeys(header, len(header), []string{"updated"}, []int{2})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(excluded, []int{0, 1}) {
		t.Fatalf("excluded = %#v", excluded)
	}
	if !isIdentityKey([]int{0, 1, 2}, 3) || isIdentityKey([]int{1, 0, 2}, 3) {
		t.Fatal("identity key classification is wrong")
	}
}
