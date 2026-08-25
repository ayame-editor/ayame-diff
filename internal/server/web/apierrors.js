// Failure messages for the GUI (#94). The server answers with a stable code
// beside its English message; this module turns the code — or, for an older or
// unclassified failure, the HTTP status — into the i18n key of a sentence that
// says what went wrong and what to do about it. A raw syscall, strconv, or
// encoding/json string never has to reach the user.
//
// Pure on purpose, so it runs under node --test (#139).
(function (root, factory) {
  const api = factory();
  if (typeof module === "object" && module.exports) module.exports = api;
  else root.AyameAPIErrors = api;
})(typeof globalThis !== "undefined" ? globalThis : this, function () {
  "use strict";

  const KEY_BY_CODE = {
    file_not_found: "errFileNotFound",
    permission_denied: "errPermissionDenied",
    invalid_path: "errInvalidPath",
    invalid_request: "errInvalidRequest",
    overwrite_refused: "errOverwriteRefused",
    unsupported_input: "errUnsupportedInput",
    timeout: "errTimeout",
    busy: "errBusy",
    unauthorized: "errUnauthorized",
    internal: "errInternal",
  };

  const KEY_BY_STATUS = {
    401: "errUnauthorized",
    403: "errPermissionDenied",
    404: "errFileNotFound",
    408: "errTimeout",
    413: "errUnsupportedInput",
    429: "errBusy",
  };

  // Returns the i18n key for a failure, or "" when nothing better than the
  // server's own message is known — the caller then shows that message rather
  // than inventing a wrong explanation.
  function apiErrorKey(code, status) {
    const byCode = KEY_BY_CODE[String(code || "")];
    if (byCode) return byCode;
    const byStatus = KEY_BY_STATUS[Number(status)];
    if (byStatus) return byStatus;
    if (Number(status) >= 500) return "errInternal";
    return "";
  }

  return { apiErrorKey, KEY_BY_CODE, KEY_BY_STATUS };
});
