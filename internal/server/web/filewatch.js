(function (root, factory) {
  const api = factory();
  if (typeof module === "object" && module.exports) module.exports = api;
  else root.AyameFileWatch = api;
})(typeof globalThis !== "undefined" ? globalThis : this, function () {
  "use strict";

  function watchPathsForMode(mode, fields, scratch) {
    if (scratch) return [];
    let names;
    if (mode === "threeway" || mode === "threeway-csv") {
      names = ["base", "old", "new"];
    } else if (mode === "text" || mode === "sorted" || mode === "csv") {
      names = ["old", "new"];
    } else {
      return [];
    }
    const seen = new Set();
    const paths = [];
    for (const name of names) {
      const path = String(fields?.[name] || "").trim();
      if (path && !seen.has(path)) {
        seen.add(path);
        paths.push(path);
      }
    }
    return paths;
  }

  // One watcher owns one authenticated long-poll request at a time. A timeout
  // response simply renews the request with the returned snapshot; a change
  // hands control back to the application, which re-compares and then starts a
  // fresh watcher. stop() aborts immediately when paths, mode, or preference
  // changes.
  function createLongPollWatcher({ request, onChange, onError }) {
    let generation = 0;
    let controller = null;
    let running = false;

    function stop() {
      generation++;
      running = false;
      controller?.abort();
      controller = null;
    }

    function start(paths, baseline) {
      stop();
      if (!paths?.length || !baseline?.length) return false;
      const ownGeneration = generation;
      const ownController = new AbortController();
      controller = ownController;
      running = true;

      (async () => {
        let snapshot = baseline;
        try {
          while (ownGeneration === generation) {
            const response = await request(paths, snapshot, ownController.signal);
            if (ownGeneration !== generation) return;
            snapshot = response.snapshot;
            if (response.changed?.length) {
              await onChange({
                paths: [...paths],
                changed: [...response.changed],
                snapshot,
              });
              return;
            }
          }
        } catch (error) {
          if (error?.name !== "AbortError" && ownGeneration === generation) {
            onError?.(error);
          }
        } finally {
          if (ownGeneration === generation) {
            running = false;
            controller = null;
          }
        }
      })();
      return true;
    }

    return {
      start,
      stop,
      isRunning: () => running,
    };
  }

  return { watchPathsForMode, createLongPollWatcher };
});
