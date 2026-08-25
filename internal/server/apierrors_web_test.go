package server

import (
	"strings"
	"testing"
)

// TestAPIErrorAssetsAreWired guards the browser half of #94: every failing API
// call must go through the mapper, so a raw syscall, strconv, or encoding/json
// string cannot reach the user.
func TestAPIErrorAssetsAreWired(t *testing.T) {
	t.Parallel()

	index := readWebAsset(t, "index.html")
	app := readWebAsset(t, "app.js")

	if !strings.Contains(index, `<script src="apierrors.js"></script>`) {
		t.Error("index.html does not load apierrors.js")
	}
	if strings.Index(index, `src="apierrors.js"`) > strings.Index(index, `src="app.js"`) {
		t.Error("apierrors.js must load before app.js")
	}
	for _, want := range []string{
		"globalThis.AyameAPIErrors",
		"function apiError(body, response)",
		"function apiErrorLocation(body)",
		`apiErrorKey(body?.code, response?.status)`,
	} {
		if !strings.Contains(app, want) {
			t.Errorf("app.js is missing failure-message wiring %q", want)
		}
	}
	for _, line := range strings.Split(app, "\n") {
		if strings.Contains(line, "new Error(") && strings.Contains(line, ".error ||") {
			t.Errorf("a server message is shown unmapped: %s", strings.TrimSpace(line))
		}
	}
}

// Each code the server can answer with needs a sentence in both languages, and
// each sentence has to carry a remedy rather than only naming the failure.
func TestFailureMessagesExistInBothLanguagesWithARemedy(t *testing.T) {
	t.Parallel()

	app := readWebCatalog(t, "app.js")
	module := readWebAsset(t, "apierrors.js")

	for _, code := range []string{
		CodeFileNotFound, CodePermissionDenied, CodeInvalidPath, CodeInvalidRequest,
		CodeOverwriteRefused, CodeUnsupportedInput, CodeTimeout, CodeBusy,
		CodeUnauthorized, CodeInternal,
	} {
		if !strings.Contains(module, code+":") {
			t.Errorf("apierrors.js does not map the %q code", code)
		}
	}
	for _, key := range []string{
		"errFileNotFound", "errPermissionDenied", "errInvalidPath", "errInvalidRequest",
		"errOverwriteRefused", "errUnsupportedInput", "errTimeout", "errBusy",
		"errUnauthorized", "errInternal", "errHTTP", "errorAt",
	} {
		if strings.Count(app, key+":") < 2 {
			t.Errorf("%s is not defined in both language tables", key)
		}
	}
	// A remedy is a second sentence. Naming the failure alone leaves the user
	// exactly where the raw error did.
	for _, sentence := range []string{
		`errFileNotFound: "ファイルが見つかりません。`,
		`errPermissionDenied: "アクセス権限がありません。`,
		`errTimeout: "処理が時間切れになりました。`,
	} {
		if !strings.Contains(app, sentence) {
			t.Errorf("app.js is missing the localized failure sentence %q", sentence)
		}
	}
	// The field list used to reach the user as a literal "{fields}".
	if strings.Contains(app, `needPaths: "{fields}`) || strings.Contains(app, `needPaths: "Specify {fields}"`) {
		t.Error("needPaths still uses an uninterpolated placeholder")
	}
}
