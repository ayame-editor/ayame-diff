package server

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/ayame-editor/ayame-diff/internal/engine"
)

// maxSuggestCandidates bounds one suggestion request. Each candidate is a full
// engine run, so the cap is what keeps "what would clear this?" from turning a
// single click into an unbounded number of comparisons (#121). It matches the
// client-side cap in web/suggest.js.
const maxSuggestCandidates = 8

// csvSuggestCandidate is one option set the client wants evaluated. The request
// is a complete csvRequest (the client builds it by applying an option patch),
// so no partial-merge ambiguity survives into the engine.
type csvSuggestCandidate struct {
	ID      string     `json:"id"`
	Request csvRequest `json:"request"`
}

type csvSuggestRequest struct {
	Candidates []csvSuggestCandidate `json:"candidates"`
}

type csvSuggestResult struct {
	ID              string         `json:"id"`
	DifferenceCount int            `json:"difference_count"`
	Summary         engine.Summary `json:"summary"`
	Error           string         `json:"error,omitempty"`
}

// handleCSVSuggest evaluates a bounded set of candidate option sets and
// reports the differences each leaves behind. It reuses the normal comparison
// path (csvConfig + engine.Run) so a suggestion's residual is exactly what the
// next Compare would show, and it is gated by the same concurrency limit as any
// other comparison.
func (s *Server) handleCSVSuggest(w http.ResponseWriter, r *http.Request) {
	req, ok := decodePostJSON[csvSuggestRequest](w, r, "")
	if !ok {
		return
	}
	if len(req.Candidates) == 0 || len(req.Candidates) > maxSuggestCandidates {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("candidates must contain between 1 and %d entries", maxSuggestCandidates))
		return
	}
	dir, err := os.MkdirTemp("", "ayame-diff-suggest-")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer os.RemoveAll(dir)

	results := make([]csvSuggestResult, 0, len(req.Candidates))
	for index, candidate := range req.Candidates {
		result := csvSuggestResult{ID: candidate.ID}
		if candidate.Request.Old == "" || candidate.Request.New == "" {
			result.Error = "both left and right paths are required"
			results = append(results, result)
			continue
		}
		if candidate.Request.KeyMode == "include" && len(candidate.Request.KeyNames)+len(candidate.Request.KeyIndexes) == 0 {
			result.Error = "select at least one key column"
			results = append(results, result)
			continue
		}
		output := filepath.Join(dir, fmt.Sprintf("candidate-%d.jsonl", index))
		summary, err := engine.Run(r.Context(), csvConfig(candidate.Request, output))
		if err != nil {
			result.Error = err.Error()
			results = append(results, result)
			continue
		}
		result.Summary = summary
		result.DifferenceCount = summaryDifferenceCount(summary)
		results = append(results, result)
	}
	writeJSON(w, http.StatusOK, struct {
		Results []csvSuggestResult `json:"results"`
	}{results})
}

// summaryDifferenceCount is the number of distinct differences a summary
// describes, matching the csv/diff response's difference_count: a changed row
// pair counts once (so the two sides are not double counted), plus the
// one-sided rows.
func summaryDifferenceCount(summary engine.Summary) int {
	changed := max(summary.ChangedLeft, summary.ChangedRight)
	return int(changed + summary.LeftOnly + summary.RightOnly)
}
