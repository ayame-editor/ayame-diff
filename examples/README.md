# Examples

Sample inputs for trying ayame-diff quickly.

## `merge-demo/` — merge-mode toggle demo

A small OLD/NEW text pair that produces several hunks (one insertion and a few
replacements), so you can see the opt-in **merge mode** from #100 in action:
the per-hunk "use left / use right" adopt buttons stay hidden while you read the
diff and appear only after you turn merge mode on.

### View the diff in the terminal

```sh
ayame-diff text --side-by-side examples/merge-demo/old.txt examples/merge-demo/new.txt
```

### Try merge mode in the browser GUI

Start the local server, then open the URL below. `autorun=1` runs the
comparison immediately; the GUI needs absolute paths, so `$PWD` is used to
build them:

```sh
ayame-diff serve --addr 127.0.0.1:8080 &
open "http://127.0.0.1:8080/?old=$PWD/examples/merge-demo/old.txt&new=$PWD/examples/merge-demo/new.txt&mode=text&autorun=1"
```

Once the diff is shown, click **Merge mode** (top of the diff navigation) to
reveal the per-hunk adopt buttons and the merge panel. Every fresh comparison
starts back in reading mode.

> Replace `open` with `xdg-open` on Linux, or just paste the URL into a browser.
> When you are done, stop the background server with `kill %1`.
