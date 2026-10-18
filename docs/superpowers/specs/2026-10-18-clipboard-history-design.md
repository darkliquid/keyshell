# Clipboard History — Design

Status: Draft for review (not yet implemented)
Date: 2026-10-18

## 1. Problem

keyshell can launch, switch, search and control the system, but it cannot show
you what you copied five minutes ago. Clipboard history is the last core launcher
feature it lacks, and it is the one feature that needs something the shell has
never had: a process that keeps running while the launcher is hidden.

## 2. Decisions

| Question | Decision |
| --- | --- |
| Secret policy | **Explicit controls, no heuristics.** Cap, TTL, a pause toggle and a clear action. No app-name skip list, because the usual ways to copy a password (a terminal running `pass -c`, or a browser's own manager) are exactly what such a list misses. Documented as best-effort, never as a guarantee. |
| Merged search | **Not searchable.** Reachable only by its trigger or `Tab`, so nothing you copied can surface in the default query. |
| Capture mechanism | `wl-paste --watch`, verified working on niri. |
| Watcher lifetime | **Shell lifetime**, not mode-scoped; restarted on unexpected exit. |
| Pause state | **The watcher process itself**, not a flag file. |
| Where the logic lives | A helper script run per capture, so the retention and validation rules are directly testable outside QML. |

## 3. Components and boundaries

### New files

| File | Kind | Responsibility |
| --- | --- | --- |
| `ClipboardBackend.qml` | Singleton | backend contract; owns the watcher `Process`; reads the store; serves items; toggles pause |
| `clipboard-watch.py` | helper, one process per change | reads the selection on stdin, validates it, dedupes, prunes, rewrites the store atomically |
| `components/ClipboardCardDelegate.qml` | Component | text preview plus relative age |

### Changed files

- **`Config.qml`** — entry cap, TTL minutes, maximum entry bytes, and the mode's
  key bindings.
- **`components/LauncherWindow.qml`** — mode keys: `X` delete the selected entry,
  `C` clear the history, `P` pause or resume capture.
- **`BackendManager.qml`** — add the backend to `builtinBackends`. Nothing else
  is needed: result collection is registry-driven.
- **`AGENTS.md`** — the policy, stated as best-effort, plus the new rule that a
  process may outlive the launcher.
- **`README.md`** — user-facing description and the honest caveat.

## 4. Capture pipeline

- The watcher command is
  `["wl-paste", "--type", "text", "--watch", <helper path>]`, started once when
  the shell starts. The helper path is resolved from `Quickshell.shellDir`, so it
  works from any checkout.
- `--watch` fires once at start with whatever is already on the clipboard, so the
  helper dedupes rather than assuming every invocation is new content.
- The helper reads stdin, then exits. It rejects empty payloads, payloads above
  `Config.maxClipboardEntryBytes`, and anything that is not valid UTF-8.
- The store is `${XDG_STATE_HOME:-~/.local/state}/keyshell/clipboard.json`, the
  same convention frecency already uses: an array of `{text, copiedAt}`, newest
  first. Writes go to a temporary file and are renamed into place, so a crash
  mid-write cannot corrupt it.
- Pruning by `Config.clipboardTtlMinutes` and `Config.maxClipboardEntries` happens
  on write (in the helper) and on read (in the backend).
- Named pipe style buffers are not used: nothing accumulates between captures.
- The watcher is deliberately **not** stopped when the launcher is dismissed. It
  is the one process whose whole purpose is to run while the overlay is hidden,
  so `Launcher.dismiss()` must leave it alone, unlike plugin processes.

## 5. Policy, stated honestly

Nothing here can tell a password from a shopping list. What the feature provides
is bounded exposure and explicit control:

- entries expire after `Config.clipboardTtlMinutes`;
- only the newest `Config.maxClipboardEntries` are kept;
- `P` stops capture entirely, and it is the watcher's own process state, so there
  is no on-disk flag that can disagree with reality;
- `C` clears the history, and `X` removes a single entry, which matters because a
  single bad capture should not force you to discard everything;
- nothing is recorded beyond the text and a timestamp: not the source
  application, not a hash, nothing derived.

Explicitly not attempted: detecting secrets, skipping password managers by window
class, or any claim that copied passwords are kept out of the history.

## 6. Behaviour

- **Mode**: badge `CLIP`, trigger `,cb`, `searchable: false`, `trackable: false`.
- **Items**: `id` is `"clip:" + copiedAt`, unique per entry and stable within a
  session without needing a content hash. `title` is the entry text (elided by
  the delegate), `subtitle` is a relative age such as "2 minutes ago", `glyph` is
  📋, and the primary action copies the text back to the clipboard via
  `execDetached(["wl-copy", text])`. Copying an older entry back promotes it to
  the top, which is the dedupe rule doing its job rather than a special case.
- **Keys** in this mode: `X` deletes the selected entry, `C` clears all, `P`
  toggles capture.
- **Empty history** shows nothing rather than an error, and the status hint
  says the history is empty or that capture is paused, whichever applies.

## 7. Errors and degradation

- `wl-paste` missing or the watcher exiting unexpectedly: the watcher is
  restarted once with a short delay, and after repeated failures the mode reports
  that capture is unavailable instead of retrying forever.
- An unreadable or malformed store: it is quarantined to `clipboard.json.bad` and
  an empty history is started, matching how the frecency store already behaves.
- A failed copy back to the clipboard leaves the history untouched.

## 8. Config additions

- `clipboardTrigger` — `,cb`. (Kept in Config so the trigger is not scattered.)
- `maxClipboardEntries` — the cap.
- `clipboardTtlMinutes` — the age limit.
- `maxClipboardEntryBytes` — the per-entry size limit.

## 9. Testing

- `tests/run.js` pipes known text through `clipboard-watch.py` and asserts: a new
  entry is stored; identical consecutive content is deduped; the cap is enforced;
  expired entries are pruned; oversized and empty payloads are rejected; the file
  is rewritten atomically and stays valid JSON. Most of the feature's logic is
  here, so most of it gets real tests.
- End to end with `wdotool` and the `isOpen()` oracle: copy known text, open
  `,cb`, press Enter and confirm with `wl-paste` that the text was copied back;
  pause, copy something new, and confirm it was **not** stored; clear, and confirm
  the history is empty. Every clipboard invocation is wrapped in a timeout,
  because `wl-copy` parks in the foreground to serve the selection.

## 10. Out of scope

- Images and other non-text clipboard types.
- Pinning entries, or a favourites concept.
- Recording the source application, or any metadata beyond text and time.
- Secret detection of any kind.
- A per-entry editor.

## 11. Open questions

- Whether capture should also be stoppable from IPC, for a compositor keybind.
- Whether the cap and TTL defaults (50 entries, 24 hours) suit real use; they are
  Config values and easy to change.
- Whether the watcher should also be paused automatically while a screen lock is
  active, which needs a lock-state signal the shell does not read today.