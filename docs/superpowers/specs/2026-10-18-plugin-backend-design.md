# Plugin / Scriptable Backend — Design

Status: Draft for review (not yet implemented)
Date: 2026-10-18

## 1. Problem

keyshell has a proven backend contract: `id`, `name`, `badge`, `prefix`,
`searchable`, `trackable`, `cardDelegate`, `search()`, `refresh()` and a
`resultsReady(query, items)` signal. Calculus and run-command were added by
satisfying that contract with no changes to the shell's plumbing, and merged
search, frecency and per-source tagging all worked without plugin-specific code.

What is missing is a way for a backend to be authored **outside this repository**,
in any language, without touching QML. This spec defines that plugin interface.

## 2. Decisions

| Question | Decision |
| --- | --- |
| How plugins appear as modes | **One backend per plugin** — its own badge, Tab stop and trigger. |
| How a plugin mode is triggered | **Multi-character trigger** matched against the whole query, longest match wins. |
| What a plugin contributes to the UI | **Data plus a glyph and badge hint**, rendered by one generic card. No plugin QML is loaded in-process. |
| Plugin process lifetime | **Alive while its own mode is open**, killed on mode change and on dismiss. |
| Participation in merged search | **Opt-in per manifest** via `searchable`; a searchable plugin is spawned per merged query. |
| Structure | **Approach A**: a registry discovers manifests and instantiates one `PluginBackend` component per plugin. |
| Action model | **Declarative actions only** — the shell performs a small, auditable verb set. |

## 3. Component boundaries

### New files

| File | Kind | Responsibility |
| --- | --- | --- |
| `PluginRegistry.qml` | Singleton | discovers manifests, owns the `Instantiator` that creates one `PluginBackend` per plugin, exposes the live list as `backends` |
| `PluginBackend.qml` | `QtObject` component, **not** a singleton | one instance per plugin: the backend contract plus its own `Process` |
| `components/PluginCardDelegate.qml` | Component | the single generic card: glyph, title, subtitle, badge hint |
| `plugin-proto.js` | plain JS, no QML imports | `parseLines()`, `normalizeItem()`, `matchTrigger()`; node-testable like `rank.js` and `calc.js` |

### Changed files

- **`BackendManager.qml`** — `backends` becomes built-ins concatenated with
  `PluginRegistry.backends`, so it must be a change-notifying property rather
  than a fixed array. `checkPrefixTrigger` is replaced by whole-query matching
  with longest-match-wins.
- **`components/LauncherWindow.qml`** — the key path (around line 537) currently
  tests only `event.text` against prefixes; it must offer the whole prospective
  query instead.
- **`Launcher.qml`** — `dismiss()` stops every plugin process, per the existing
  resource-cleanup rule.
- **`Config.qml`** — plugin search directories, process timeout, merged-query
  debounce, plugin result cap.
- **`AGENTS.md` / `README.md`** — document the plugin contract and trigger syntax.

The point of this shape: a plugin instance satisfies the same interface as a
built-in backend, so merged search, frecency tagging, Tab cycling and prefix
dispatch acquire no plugin-specific branches. The `Instantiator` already present
in `BackendManager.qml` is model-driven, so it creates the result `Connections`
for runtime-added backends automatically.

## 4. Discovery and manifest

Each plugin is a directory containing a `plugin.json`. Directories are searched
in order:

1. `$XDG_CONFIG_HOME/keyshell/plugins`
2. `$XDG_DATA_HOME/keyshell/plugins`
3. each `$XDG_DATA_DIRS/keyshell/plugins`

Discovery runs at shell startup. A plugin added later appears after a restart;
live rescans are out of scope for this cut.

```json
{
  "name": "Docker",
  "badge": "DOCKER",
  "trigger": ";docker",
  "command": ["/usr/bin/docker-plugin"],
  "glyph": "🐳",
  "searchable": false,
  "trackable": true,
  "description": "Containers and images"
}
```

- `trigger` must be at least **2 characters**, which guarantees it can never
  collide with a built-in single-character sigil.
- `searchable` defaults to `false`; `trackable` defaults to `false`.
- A manifest that fails to parse, lacks a `name`, `trigger` or `command`, or
  whose `trigger` is shorter than 2 characters is skipped and logged once.

## 5. Protocol

The shell spawns `command` and writes one JSON line per request to stdin:

```json
{"query": "ps", "limit": 60, "merged": false}
```

`merged` is `true` when the request comes from the merged default query and
`false` when it comes from the plugin's own mode. A searchable plugin needs it:
the same `search()` call serves both, so without the flag a plugin cannot tell
an empty merged query from an empty query in its own mode. The usual shape is to
return nothing for an empty merged query, so opening the launcher is never
flooded, while listing its busiest entries for an empty query in its own mode.

The plugin writes one JSON object per line to stdout, and terminates each
response with the line `{"done": true}`, then stays alive while its mode is open.
The shell collects items per line and emits when it sees the terminator, or when
`Config.pluginTimeoutMs` elapses; the terminator is what makes the end of a
response deterministic rather than guessed.

The shell keeps at most one request in flight. If a query arrives while a
response is still outstanding, it waits and is sent once that response completes,
and only the newest pending query is kept. A plugin therefore sees strict
request, response, request, response, and never has to correlate responses with
requests. Item schema:

```json
{
  "id": "abc",
  "title": "web-1",
  "subtitle": "running",
  "glyph": "🐳",
  "badge": "UP",
  "destructive": true,
  "actions": [{"type": "exec", "argv": ["docker", "rm", "-f", "web-1"]}]
}
```

`destructive` is optional and defaults to `false`. When it is `true` the shell
refuses to run the item's primary action on a plain Enter: the item needs the
same confirmation the power actions use, which is a hold of
`Config.powerHoldDurationMs` or a double-tap. The plugin still owns the
decision to be destructive; the shell owns the confirmation.

`plugin-proto.js` validates every line and drops anything malformed, so a broken
plugin cannot take the shell down.

## 6. Action model

Actions are declarative. The shell implements this verb set and nothing else:

| Type | Fields | Shell behaviour |
| --- | --- | --- |
| `copy` | `text` | copy to the clipboard via `wl-copy` |
| `open-url` | `url` | `xdg-open` |
| `exec` | `argv` | run detached |
| `exec-terminal` | `argv` | run in the resolved terminal |
| `set-query` | `query` | replace the query with `query` |

The confirmation for a `destructive` item is not an action type: it is a gate on
the whole item, so any of these verbs can sit behind it.

An action with an unknown `type`, a missing required field, or a non-array
`argv` is rejected when the line is parsed, and the item keeps its other valid
actions (or is dropped if none remain).

Plugins never receive code execution inside the shell process. Executing an
`exec` action is the same trust level as running the plugin executable itself,
which the user installed deliberately.

`exec-terminal` needs the same terminal resolution the run-command backend
already performs (`$TERMINAL`, else `ghostty`, else `alacritty`), so that probe
is extracted into a shared place both consume rather than duplicated.

## 7. Trigger matching

`matchTrigger(query, triggers)` returns the longest matching trigger and the
remaining query text.

- Longest match wins, so `;docker` beats a hypothetical `;d`.
- A trigger matches only at the **start** of the query; `;docker` is a match for
  `;docker ps` but not for `x;docker`.
- Because plugin triggers are at least 2 characters and built-in sigils are
  exactly 1, typing `=` still opens the calculator and can never be swallowed.
- `LauncherWindow` passes the whole prospective query (current query plus the new
  keystroke) to `BackendManager.tryTrigger()`; when a trigger matches, the mode
  switches and the query becomes the remainder.
- Re-matching on Backspace (stepping back out of a plugin mode) is not required
  in this cut; Escape and Backspace leave the mode as they do today.

## 8. Merged search

A plugin declaring `searchable: true` is spawned per merged query, debounced by
`Config.pluginQueryDebounceMs` alongside file search, and guarded by the usual
generation counter so stale answers are discarded. Its items are tagged with
`source = plugin id`, so frecency and per-source tagging work unchanged.

A searchable plugin is therefore spawned per merged query even though a
non-searchable plugin is only alive while its own mode is open.

## 9. Lifecycle and errors

- Mode entry: `refresh()` starts the process if needed and writes the request.
- Mode exit (`setMode` away) and `Launcher.dismiss()`: the process is stopped.
- Timeout: a plugin exceeding `Config.pluginTimeoutMs` is killed, and its mode
  reports itself unavailable rather than hanging the reel.
- Non-zero exit or a missing binary marks the plugin unavailable the same way;
  its mode still exists so the failure is discoverable.
- A crash restarts on next mode entry, logged once.

## 10. Config additions

- `pluginSearchDirs` — the ordered discovery directories.
- `pluginTimeoutMs` — per-request timeout.
- `pluginQueryDebounceMs` — debounce for searchable plugins in merged search.
- `maxPluginResults` — per-plugin result cap.
- `minTriggerLength` — the minimum trigger length (2).

## 11. Testing

- `plugin-proto.js` gets node assertions in `tests/run.js`: a valid line
  normalises, a malformed line is dropped, an unknown action type is rejected, an
  item missing `id` or `title` is dropped, longest-match trigger wins,
  single-character built-ins are not swallowed, and the remainder is extracted
  correctly.
- A fixture plugin (a small shell script under `tests/fixtures/`) exercises the
  process protocol end to end, with an **unshifted** trigger such as `,fx` so the
  whole path stays drivable by `wdotool key`.
- Runtime verification uses the established method: the `isOpen()` IPC oracle,
  click-to-focus for `OnDemand` keyboard focus, and `wdotool key` for unshifted
  keys only. The shifted `;` trigger will be verified only as far as the input
  path allows.

## 12. Out of scope

- Live rescan of the plugin directories.
- Plugin-authored QML cards or console views.
- Sandboxing or resource isolation of plugin processes.
- Install / uninstall / management commands for plugins.
- Non-JSON protocols.
- Plugin triggers shorter than two characters.

## 13. Open questions

- Whether discovery should also react to filesystem changes within a session.
- Whether a plugin should be able to declare multiple triggers.
- Whether `trackable` plugins should record frecency under a plugin-scoped key
  when their item ids are not stable across runs.