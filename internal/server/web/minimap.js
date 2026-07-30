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

  const api = { calculateMinimapViewport, scrollTopForMinimapPointer };
  root.AyameMinimap = api;
  if (typeof module !== "undefined" && module.exports) module.exports = api;
})(typeof globalThis !== "undefined" ? globalThis : window);
