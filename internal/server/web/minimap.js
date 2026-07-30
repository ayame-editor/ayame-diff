// Geometry shared by the difference minimap's rendering and drag handling.
//
// The result pane is its own scroll container. Keeping these calculations
// outside app.js makes that coordinate system explicit and lets node exercise
// the edge cases without a browser DOM (#102).
(function (root) {
  "use strict";

  function clamp(value, minimum, maximum) {
    return Math.max(minimum, Math.min(maximum, value));
  }

  function calculateMinimapViewport({
    scrollTop = 0,
    scrollHeight = 0,
    clientHeight = 0,
    minimumFraction = 0.03,
  } = {}) {
    const contentHeight = Math.max(0, Number(scrollHeight) || 0);
    const visibleHeight = Math.max(0, Number(clientHeight) || 0);
    const maxScrollTop = Math.max(0, contentHeight - visibleHeight);
    if (!contentHeight || !visibleHeight || !maxScrollTop) {
      return { scrollable: false, top: 0, height: 1, maxScrollTop: 0 };
    }

    const height = clamp(
      visibleHeight / contentHeight,
      Math.max(0, Number(minimumFraction) || 0),
      1,
    );
    const progress = clamp((Number(scrollTop) || 0) / maxScrollTop, 0, 1);
    return {
      scrollable: true,
      top: progress * (1 - height),
      height,
      maxScrollTop,
    };
  }

  function scrollTopForMinimapPointer({
    pointerY = 0,
    trackTop = 0,
    trackHeight = 0,
    viewportHeight = 0,
    grabOffset = 0,
    scrollHeight = 0,
    clientHeight = 0,
  } = {}) {
    const maxScrollTop = Math.max(
      0,
      (Number(scrollHeight) || 0) - (Number(clientHeight) || 0),
    );
    const travel = Math.max(
      0,
      (Number(trackHeight) || 0) - (Number(viewportHeight) || 0),
    );
    if (!maxScrollTop || !travel) return 0;

    const viewportTop = clamp(
      (Number(pointerY) || 0) - (Number(trackTop) || 0) - (Number(grabOffset) || 0),
      0,
      travel,
    );
    return (viewportTop / travel) * maxScrollTop;
  }

  const markerPriorities = Object.freeze({
    ignored: 0,
    insert: 1,
    delete: 2,
    replace: 3,
    moved: 4,
    conflict: 5,
  });

  function minimapMarkerPriority(marker) {
    if (marker?.ignored) return markerPriorities.ignored;
    if (marker?.kind === "conflict") return markerPriorities.conflict;
    if (marker?.moved) return markerPriorities.moved;
    return markerPriorities[marker?.kind] ?? markerPriorities.replace;
  }

  // Collapse logical hunk ranges into at most one winner per track pixel.
  //
  // The result pane renders only hunks, so source line numbers are the wrong
  // coordinate system: a large unchanged gap does not occupy any result-space,
  // while a large displayed hunk does. displayLength keeps the map aligned with
  // the rows the user can actually scroll through. Painting into a bounded
  // pixel buffer also prevents thousands of events from creating thousands of
  // buttons. When two ranges quantize to the same pixel, the important state
  // wins deterministically instead of whichever marker happened to be later in
  // the DOM (#265).
  function calculateMinimapSegments(markers = [], trackPixels = 1) {
    const pixelCount = Math.max(1, Math.floor(Number(trackPixels) || 1));
    const entries = markers.map((marker, order) => ({
      ...marker,
      index: Number.isInteger(marker?.index) ? marker.index : order,
      displayLength: Math.max(1, Number(marker?.displayLength) || 1),
      priority: minimapMarkerPriority(marker),
    }));
    if (!entries.length) return [];

    // One unit accounts for the hunk header. The rest represents its displayed
    // rows, including the longer side of a replacement.
    const totalUnits = entries.reduce((sum, entry) => sum + entry.displayLength + 1, 0);
    const pixels = new Array(pixelCount).fill(null);
    let offset = 0;
    for (const entry of entries) {
      const units = entry.displayLength + 1;
      const start = clamp(Math.floor((offset / totalUnits) * pixelCount), 0, pixelCount - 1);
      const end = clamp(
        Math.max(start + 1, Math.ceil(((offset + units) / totalUnits) * pixelCount)),
        start + 1,
        pixelCount,
      );
      for (let pixel = start; pixel < end; pixel++) {
        if (!pixels[pixel] || entry.priority > pixels[pixel].priority) {
          pixels[pixel] = entry;
        }
      }
      offset += units;
    }

    const segments = [];
    for (let start = 0; start < pixelCount;) {
      const winner = pixels[start];
      if (!winner) {
        start++;
        continue;
      }
      let end = start + 1;
      while (end < pixelCount && pixels[end] === winner) end++;
      segments.push({
        ...winner,
        top: start / pixelCount,
        height: (end - start) / pixelCount,
      });
      start = end;
    }
    return segments;
  }

  const api = {
    calculateMinimapSegments,
    calculateMinimapViewport,
    minimapMarkerPriority,
    scrollTopForMinimapPointer,
  };
  root.AyameMinimap = api;
  if (typeof module !== "undefined" && module.exports) module.exports = api;
})(typeof globalThis !== "undefined" ? globalThis : window);
