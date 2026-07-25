// Logical scroll anchors for result re-renders.
//
// Saving scrollTop is not enough when comparison options add or remove rows:
// the same pixel offset then points at different content. Renderers instead
// mark stable logical items (a source line, difference id, or path), and this
// module preserves the marked item plus its offset inside the viewport (#249).
//
// Kept outside app.js so the selection and fallback rules run under node
// without a browser DOM. The small fake-DOM surface needed by the tests is the
// same surface used here: querySelectorAll, dataset, getBoundingClientRect, and
// scrollTop.
(function (root) {
  "use strict";

  const ANCHOR_SELECTOR = "[data-scroll-anchor][data-scroll-key]";

  function anchors(container, group) {
    if (!container?.querySelectorAll) return [];
    return Array.from(container.querySelectorAll(ANCHOR_SELECTOR))
      .filter((node) => group == null || node.dataset.scrollAnchor === group);
  }

  function numericOrder(node) {
    const order = Number(node?.dataset?.scrollOrder);
    return Number.isFinite(order) ? order : null;
  }

  // captureScrollAnchor chooses the first partially or fully visible logical
  // item. topInset excludes a sticky pane header from the readable viewport.
  function captureScrollAnchor(container, topInset = 0) {
    if (!container?.getBoundingClientRect) return null;
    const containerRect = container.getBoundingClientRect();
    const viewportTop = containerRect.top + Math.max(0, Number(topInset) || 0);
    const viewportBottom = containerRect.bottom;
    for (const node of anchors(container)) {
      const rect = node.getBoundingClientRect();
      if (rect.bottom <= viewportTop || rect.top >= viewportBottom) continue;
      return {
        group: node.dataset.scrollAnchor,
        key: node.dataset.scrollKey,
        order: numericOrder(node),
        offset: rect.top - viewportTop,
      };
    }
    return null;
  }

  // findScrollAnchor first uses the stable key. If a comparison change removes
  // that item, it picks the nearest surviving logical order in the same group;
  // ties choose the earlier item, then DOM order, making the fallback stable.
  function findScrollAnchor(container, anchor) {
    if (!anchor?.group) return null;
    const candidates = anchors(container, anchor.group);
    const exact = candidates.find((node) => node.dataset.scrollKey === anchor.key);
    if (exact) return exact;
    if (!Number.isFinite(anchor.order)) return null;

    let best = null;
    let bestDistance = Infinity;
    let bestOrder = Infinity;
    for (const node of candidates) {
      const order = numericOrder(node);
      if (order == null) continue;
      const distance = Math.abs(order - anchor.order);
      if (distance < bestDistance || (distance === bestDistance && order < bestOrder)) {
        best = node;
        bestDistance = distance;
        bestOrder = order;
      }
    }
    return best;
  }

  function restoreScrollAnchor(container, anchor, topInset = 0) {
    if (!container?.getBoundingClientRect) return false;
    const target = findScrollAnchor(container, anchor);
    if (!target) return false;
    const containerRect = container.getBoundingClientRect();
    const viewportTop = containerRect.top + Math.max(0, Number(topInset) || 0);
    const currentOffset = target.getBoundingClientRect().top - viewportTop;
    container.scrollTop += currentOffset - anchor.offset;
    return true;
  }

  const api = { captureScrollAnchor, findScrollAnchor, restoreScrollAnchor };
  root.AyameScrollAnchor = api;
  if (typeof module !== "undefined" && module.exports) module.exports = api;
})(typeof globalThis !== "undefined" ? globalThis : window);
