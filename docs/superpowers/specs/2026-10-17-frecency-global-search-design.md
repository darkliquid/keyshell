# Frecency & Global Search — Design

Status: Draft for review (not yet implemented)
Date: 2026-10-17

## 1. Problem

keyshell is launcher-only, with one visible mode at a time. The no-prefix reel is
currently hardwired to Applications (`AppBackend.prefix: ""`). As backends and
user plugins multiply, two things break down:

- The user must know which mode holds the thing they want (app vs window vs
  command vs clipboard), so the "just type" path only works for apps.
- Nothing is learned from use: the same commands are retyped and the same apps
  sit in alphabetical order regardless of habit.

This spec adds a merged default search and a persisted frecency signal. It is the
foundation the plugin backend will lean on, so it is designed first.

## 2. Decisions

| Question | Decision |
| --- | --- |
| No-prefix behaviour | **Merged default.** No-prefix queries all *searchable* sources. |
| Sources in merged search | **Backend-declared `searchable` flag.** |
| Ranking model | **Match-first**, frecency as tiebreak. |
| What frecency tracks | **Backend-declared `trackable` flag.** |
| Pinning / favourites | **Out of scope** for this cut; can ride the same store later. |
| Structure | **Approach B**: `BackendManager` stays a registry; a new aggregator owns fan-out/merge/rank; a frecency singleton owns the store. |
| Tests | Add a dependency-free `tests/` harness for the pure math. |

## 3. Component boundaries

### New files

| File | Responsibility | Owns |
| --- | --- | --- |
| `Frecency.qml` (Singleton) | Persisted usage scores | the `frecency.json` store; `record()`, `score()`, `prune()`, `flush()` |
| `SearchAggregator.qml` (Singleton) | Merged fan-out + ranking | `generation` counter, per-source collection, merge/sort, emits one `resultsReady` |
| `rank.js` (plain JS, no QML imports) | Pure scoring math | `matchTier()`, `frecencyScore()`, `mergeAndSort()` |
| `tests/run.js` | Unit checks for `rank.js` | plain assertions, run with `node` or `bun` |

### Changed files

- **Backend contract** — two new readonly properties, `searchable` and
  `trackable`. No other backend change: the shell scores match from
  `title`/`subtitle`/keywords, so backends keep their current `search()` shape.
- **`BackendManager.qml`** — keeps registry, mode cycling, and prefix dispatch.
  The nine hardcoded `Connections` blocks (`BackendManager.qml:115-185`) collapse
  into one registry-driven collection that the aggregator consumes. No-prefix
  resolves to a merged sentinel mode.
- **`Launcher.qml`** — `refresh()` routes to the aggregator in merged mode, else
  to `BackendManager`. `activate()` (`Launcher.qml:322`) records the hit before
  executing. Selection becomes keyed by item identity rather than index.
- **`Config.qml`** — new tunables only (Section 10).

Item identity for frecency is `source + ":" + item.id`, because item ids are only
unique within a backend. The aggregator tags each collected item with
`source` = the emitting backend's `id`.

## 4. Backend contract additions

```qml
readonly property bool searchable: false
readonly property bool trackable: false
```

Both default to **false** so every backend opts in consciously. Per-backend for
this cut:

| Backend | searchable | trackable |
| --- | --- | --- |
| Apps | yes | yes |
| Windows | yes | yes |
| Files | yes | no |
| Audio | no | no |
| Media | no | no |
| Tray | no | no |
| Bluetooth | no | no |
| Power | no | no |
| Network | no | no |
| Calc / Clipboard / Plugins (future) | yes | yes |

Future plugins express the same two flags in their manifest, so the interface
defined here is the one plugins target.

Windows are marked `trackable` because their item id is `appId::title::idx`
(`WindowBackend.qml:126`), which is stable across sessions for "same app, same
document title". It does not key on a volatile compositor window id.

## 5. Merged query data flow

1. `Launcher.setQuery` → `refresh()`. In merged mode this calls
   `SearchAggregator.searchAll(query, limit)` instead of `BackendManager.search`.
2. The aggregator increments `generation` and fans out to every `searchable`
   backend. Synchronous backends return an array inline; asynchronous ones
   (files, plugins) emit `resultsReady(query, items)` later.
3. On collection: tag the item with `source`, compute `matchTier(query, item)`,
   read `Frecency.score(source, item.id)`, merge, sort, slice to the limit, emit
   a single `resultsReady`.
4. Async hits landing inside `Config.mergeSettleMs` join the same generation;
   later ones are discarded. Re-sorting is safe because the highlighted entry is
   tracked by item id, not by index.

## 6. Ranking (match-primary)

Sort keys, in order:

1. **Match tier** — `0` prefix, `1` word start, `2` substring,
   `3` keyword/description. Items that do not match are dropped.
2. **Frecency score** — higher first.
3. **Title length**, then **alphabetical** — deterministic, mirroring the tiering
   already present at `AppBackend.qml:42-60`.

Empty query: no match filter, ordered purely by frecency, then backend order.

The frecency boost is a tiebreak within a tier only. It can never lift a
keyword match above an exact prefix match.

## 7. Frecency store

- **Path**: `${XDG_STATE_HOME:-~/.local/state}/keyshell/frecency.json`. State, not
  config, because it is regenerable.
- **Schema**: `{ "<source>:<itemId>": { "count": int, "lastUsed": epochMs } }`.
- **Write policy**: debounced (~2s after the last hit) and on `Launcher.dismiss()`
  via Quickshell `FileView` + `JsonAdapter` (`Quickshell.Io`). Never synchronously
  per activation.
- **Score**: `count` decayed across recency buckets (hour/day/week/month weights),
  normalised to `0..1` at read time.
- **Prune**: entries older than `Config.frecencyMaxAgeDays` removed on load and
  periodically.

## 8. Selection stability

`Launcher.selectedIndex` still drives the reel, but after each merge the Launcher
re-derives the index from a retained `selectedItemId`. This is the specific fix
for async file/plugin hits arriving after first paint and shifting the highlight
under the user's finger.

## 9. Errors and degradation

- Store missing → start empty. Corrupt → warn, move aside to
  `frecency.json.bad`, start empty. Never blocks the UI.
- Unwritable store → scores kept in memory, retried on the next flush.
- Async result whose query or generation does not match the current one is
  dropped — the pattern already used at `FileBackend.qml:41`.
- A settle timer guarantees the aggregator always emits, even if a source never
  answers.
- A backend that throws in `search()` is skipped for that query and logged; it is
  not fatal to the merge.

## 10. Config additions

- `mergeSettleMs` — async collection window for a merged query.
- `maxMergedResults` — merged reel cap (distinct from per-backend
  `maxResults`).
- `mergedModeId` — the sentinel mode id for merged search (`"all"`).
- `frecencyBuckets` — recency decay buckets, each `{ maxAgeMs, weight }`.
- `frecencyCountCap` — hit count at which the count factor saturates.
- `frecencyMaxAgeDays` — prune threshold.
- `frecencyFlushDebounceMs` — write debounce.

No literal values appear outside `Config.qml`, per the project rules.

## 11. Testing

- `rank.js` and the frecency math are dependency-free, so `tests/run.js` asserts
  them directly with `node tests/run.js` (or `bun`). Cases: tier ordering,
  frecency tiebreak never outranking a tier, empty-query ordering, decay across
  buckets, prune cutoff.
- Manual QML verification: merged ordering with and without history; persistence
  across a `qs` restart; highlight stability while file results land late;
  missing/corrupt store recovery; a backend without flags still works in explicit
  mode.

## 12. Out of scope

- Pinning / favourites (deliberately deferred).
- The generic plugin/scriptable backend itself.
- The clipboard and calculator backends.
- Multi-char namespacing for plugin triggers; this spec only defines the two
  flags plugins will consume.

## 13. Open questions

- Exact decay weights and half-life — to be tuned in `Config.qml` during
  implementation.
- Whether merged mode is its own Tab stop or simply the default landing mode.

Resolved during review: `source` is a **new explicit item field** set by the
aggregator, not derived from `badge`, so identity never depends on display text.