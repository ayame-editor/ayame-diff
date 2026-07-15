// Complete code
// Update the renderHunk function to only show merge buttons when the user is in merge mode
function renderHunk(hunk, row, index) {
  if (mergeMode.getMode()) {
    // Show merge buttons when the user is in merge mode
    return `
      <div class="hunk-header">
        <span class="hunk-num">@@ ${row.lineNum}</span>
        <button class="choose-left">Left</button>
        <button class="choose-right">Right</button>
        <button class="hunk-ignore">Ignore</button>
      </div>
    `;
  } else {
    // Hide merge buttons when the user is not in merge mode
    return `
      <div class="hunk-header">
        <span class="hunk-num">@@ ${row.lineNum}</span>
      </div>
    `;
  }
}