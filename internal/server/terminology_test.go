package server

import (
	"context"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidationErrorsUseLeftRightTerminology(t *testing.T) {
	t.Parallel()

	if err := validateDiffSources(diffRequest{}); err == nil || err.Error() != "both left and right paths are required" {
		t.Fatalf("validateDiffSources error = %v", err)
	}
	if _, _, _, err := threeWayTextResult(context.Background(), threeWayTextRequest{}); err == nil || err.Error() != "base, left, and right paths are required" {
		t.Fatalf("threeWayTextResult error = %v", err)
	}
	req := httptest.NewRequest("POST", "/api/three-way/csv", nil)
	if _, err := compareThreeWayCSV(req, threeWayCSVRequest{}); err == nil || err.Error() != "base, left, and right paths are required" {
		t.Fatalf("compareThreeWayCSV error = %v", err)
	}
}

func TestInputOpenErrorsIdentifyLeftAndRight(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	left := filepath.Join(dir, "left.txt")
	if err := os.WriteFile(left, []byte("left\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, _, _, err := openRequestLines(diffRequest{Old: filepath.Join(dir, "missing-left.txt"), New: filepath.Join(dir, "missing-right.txt")})
	if err == nil || !strings.HasPrefix(err.Error(), "left: ") {
		t.Fatalf("missing left error = %v", err)
	}
	_, _, _, err = openRequestLines(diffRequest{Old: left, New: filepath.Join(dir, "missing-right.txt")})
	if err == nil || !strings.HasPrefix(err.Error(), "right: ") {
		t.Fatalf("missing right error = %v", err)
	}
}
