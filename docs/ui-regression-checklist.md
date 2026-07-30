<!-- i18n: language-switcher -->
[English](ui-regression-checklist.md) | [日本語](ui-regression-checklist.ja.md)

# GUI change regression checklist

Removing permanent controls is useful only when the result gains space and the
same tasks remain easy to find. Use this checklist for every pull request that
changes `internal/server/web/`, moves a GUI action, changes a shortcut, or
alters the setup/result layout. The pull request template links here so the
review evidence stays with the change.

## Establish the baseline

Measure before and after with the same viewport, language, comparison mode, and
sample result. Record the route and number of pointer activations for each
affected task. Opening a menu or disclosure, choosing an item, and confirming a
dialog each count as one activation; focusing or typing in an already visible
control does not.

| Common task | Required result |
|---|---|
| Run the first comparison | Must not take more activations than before |
| Re-run the current comparison | **Re-compare** remains directly visible and one click away |
| Change one input path | Must not take more activations than before |
| Change whitespace handling | Must not take more activations than before |
| Go to the next difference | Must not take more activations than before |
| Switch side-by-side / unified view | Must not take more activations than before |
| Save an edited pane | One click away once direct pane editing is available |

If a task is outside the changed surface, write `unchanged` instead of
re-measuring it. If an issue intentionally changes a route, record the old and
new counts and explain why the new route still satisfies the product goal.

## Required checks

1. **Keep frequent actions reachable.** The persistent Compare/Re-compare
   action must remain one click away. Save must also remain one click away after
   direct pane editing is introduced.
2. **Do not increase routine click counts.** Complete the table above for every
   affected task. Moving a control into a menu is a regression when it adds an
   activation to a frequent task.
3. **Verify keyboard-only operation.** Reach every changed or newly introduced
   control with Tab/Shift+Tab or the documented shortcut. Exercise it with
   Enter/Space, close temporary UI with Escape, and confirm focus returns to a
   useful place. A pointer-only route is not acceptable.
4. **Give reclaimed space to the result.** When setup or toolbar chrome shrinks,
   compare the result pane dimensions before and after. The freed area must
   enlarge the result instead of becoming unused blank space.
5. **Check normal and high DPI.** At minimum, inspect 100% and 200% scaling (or
   device pixel ratios 1 and 2). Icons must remain distinguishable, controls
   must not overlap, and the active/focus state must still be visible.
6. **Preserve documented reachability.** Update the
   [GUI setup reachability and placement policy](gui-setup-parity.md) when a
   setting or action moves. Add or update an automated test for any stable
   route, shortcut, or layout invariant that can be checked without screenshots.

## Pull request evidence

In the GUI regression section of the pull request:

- mark the change as not affecting the GUI, or complete every applicable item;
- include the before/after click-count rows that changed;
- state which keyboard path was exercised;
- state the viewport and scaling used for visual checks; and
- link screenshots or a short recording when placement, icon size, or reclaimed
  result space changed materially.

Reviewers should not accept a GUI layout change with only a screenshot of the
new state. The evidence must show that frequent work did not become harder.
