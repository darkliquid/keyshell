package launcher

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	"github.com/darkliquid/keyshell/internal/backend"
	"github.com/darkliquid/keyshell/internal/rank"
	"github.com/darkliquid/keyshell/internal/telemetry"
)

// Job is one search request. It carries the whole state the query needs, so the
// worker never touches launcher state while it runs. The context links the
// query span back to the input event that produced it.
type Job struct {
	ctx   context.Context
	gen   uint64
	query string
	mode  string
	scope []string
	depth int
}

// Result is one completed search, tagged with the generation it answers. A
// result whose generation is no longer current is a superseded keystroke and
// is dropped, so a slow plugin can never publish stale results. The span
// context lets the frame that paints the results join the same trace.
type Result struct {
	gen     uint64
	depth   int
	items   []backend.Item
	matches []rank.Match
	sc      trace.SpanContext
	partial bool
}

// Start launches the query worker. Until it is called the launcher runs
// queries inline, which is what keeps unit tests synchronous.
func (l *Launcher) Start() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.jobs != nil {
		return
	}
	l.jobs = make(chan Job, 1)
	l.resultsCh = make(chan Result, 128)
	go l.work(l.jobs, l.resultsCh)
}

// work runs queries off the UI thread. Only the newest request matters: a
// pending one is replaced rather than queued, because a merged search costs a
// plugin round trip and queuing keystrokes would make the results lag behind
// the field.
func (l *Launcher) work(jobs <-chan Job, out chan Result) {
	for job := range jobs {
		l.run(job, func(res Result) {
			select {
			case out <- res:
			default:
				select {
				case <-out:
				default:
				}
				select {
				case out <- res:
				default:
				}
			}
		})
	}
}

// Poll publishes any finished query. It must run on the UI thread; the frame
// loop calls it once per frame.
func (l *Launcher) Poll() {
	l.mu.RLock()
	ch := l.resultsCh
	l.mu.RUnlock()
	if ch == nil {
		return
	}
	for {
		select {
		case res := <-ch:
			if l.apply(res) {
				l.changed()
			}
		default:
			return
		}
	}
}

// submit queues a job for the worker, replacing any job it has not taken yet.
// The newest request always wins.
func (l *Launcher) submit(job Job) {
	l.mu.RLock()
	jobs := l.jobs
	l.mu.RUnlock()
	if jobs == nil {
		l.run(job, func(r Result) {
			if l.apply(r) {
				l.changed()
			}
		})
		return
	}
	l.mu.Lock()
	l.searching = true
	l.mu.Unlock()
	select {
	case jobs <- job:
	default:
		select {
		case <-jobs:
		default:
		}
		select {
		case jobs <- job:
		default:
		}
	}
}

// apply publishes a finished query if it still matches the current generation.
func (l *Launcher) apply(res Result) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if res.gen != l.gen || res.depth != len(l.levels) {
		return false
	}
	level := l.levels[len(l.levels)-1]

	var selectedID backend.ResultID
	if level.Selected >= 0 && level.Selected < len(level.Matches) {
		selectedID = level.Matches[level.Selected].Item.ID
	}

	level.Items = res.items
	level.Matches = res.matches
	l.results = res.matches
	if !res.partial {
		l.searching = false
	}
	if len(res.matches) == 0 {
		level.Selected = 0
		if l.panel != nil && !res.partial {
			l.closePanelLocked()
		}
	} else {
		found := false
		if selectedID != "" {
			for idx, m := range res.matches {
				if m.Item.ID == selectedID {
					level.Selected = idx
					found = true
					break
				}
			}
		}
		if !found {
			if level.Selected >= len(res.matches) {
				level.Selected = 0
			}
		}
	}
	if res.sc.IsValid() {
		l.lastQuerySC = res.sc
	}
	if item, source, ok := l.selectedItemLocked(level); ok {
		slog.Debug("selection active", "index", level.Selected, "id", item.ID, "title", item.Title, "source", source, "matches", len(res.matches))
	}
	return true
}

// LastQuerySpanContext returns the trace context of the most recently
// published query result, so a frame that paints the results can join the
// same trace.
func (l *Launcher) LastQuerySpanContext() trace.SpanContext {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.lastQuerySC
}

// jobLocked snapshots the state one query needs and bumps the generation.
func (l *Launcher) jobLocked() Job {
	l.gen++
	if l.cancelQuery != nil {
		l.cancelQuery()
	}
	ctx, cancel := context.WithCancel(context.Background())
	l.cancelQuery = cancel

	var scope []string
	mode := l.mode
	if level := l.currentLevelLocked(); level != nil {
		scope = append([]string(nil), level.Scope...)
		if len(level.Scope) > 0 && level.Source != "" {
			mode = level.Source
		}
	}
	return Job{
		ctx:   ctx,
		gen:   l.gen,
		query: l.query,
		mode:  mode,
		scope: scope,
		depth: len(l.levels),
	}
}

// run executes one query against the provider. It touches no launcher state, so
// it is safe off the UI thread.
func (l *Launcher) run(job Job, emit func(Result)) Result {
	ctx := job.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, span := telemetry.StartSpan(ctx, "keyshell.query",
		trace.WithAttributes(
			attribute.String(telemetry.Attrs.Mode, job.mode),
			attribute.String("keyshell.query_text", job.query),
			attribute.Int64("keyshell.job.gen", int64(job.gen)),
		))
	defer span.End()

	start := time.Now()
	wrappedEmit := func(r Result) {
		r.gen = job.gen
		r.depth = job.depth
		r.sc = span.SpanContext()
		if emit != nil {
			emit(r)
		}
	}

	res := l.queryMode(ctx, job.mode, job.query, job.scope, l.opts.RankOpts.Limit, wrappedEmit)
	res.gen = job.gen
	res.depth = job.depth
	res.sc = span.SpanContext()
	elapsed := time.Since(start).Seconds()
	modeAttr := metric.WithAttributes(attribute.String(telemetry.Attrs.Mode, job.mode))
	telemetry.QueryDuration().Record(ctx, elapsed, modeAttr)
	telemetry.QueryResults().Record(ctx, int64(len(res.matches)), modeAttr)
	return res
}

// QuerySync runs an ad-hoc query against the provider without touching any
// launcher state. It backs the CLI and IPC query clients.
func (l *Launcher) QuerySync(ctx context.Context, mode, text string, limit int) ([]rank.Match, error) {
	if mode != "" && mode != MergedMode {
		if _, err := l.provider.Get(mode); err != nil {
			return nil, fmt.Errorf("query backend %q: %w", mode, err)
		}
	}
	if limit <= 0 {
		limit = l.opts.RankOpts.Limit
	}
	return l.queryMode(ctx, mode, text, nil, limit, nil).matches, nil
}

// queryMode runs the query for one mode, merged or scoped, and ranks it.
func (l *Launcher) queryMode(ctx context.Context, mode, text string, scope []string, limit int, emit func(Result)) Result {
	res := Result{}
	if mode == MergedMode || mode == "" {
		return l.queryMerged(ctx, text, limit, emit)
	}
	items, err := l.queryBackend(ctx, mode, text, limit, false, scope)
	if err != nil {
		if emit != nil {
			emit(res)
		}
		return res
	}
	res.items = items
	if len(scope) > 0 && text == "" {
		matches := make([]rank.Match, len(items))
		for i, it := range items {
			matches[i] = rank.Match{Item: it, Source: mode, Tier: 0}
		}
		res.matches = matches
		if emit != nil {
			emit(res)
		}
		return res
	}
	if b, bErr := l.provider.Get(mode); bErr == nil && !b.Info().Searchable {
		matches := make([]rank.Match, len(items))
		for i, it := range items {
			matches[i] = rank.Match{Item: it, Source: mode, Tier: 0}
		}
		res.matches = matches
		if emit != nil {
			emit(res)
		}
		return res
	}
	res.matches = rank.RankOne(text, mode, items, nil, l.opts.RankOpts)
	if emit != nil {
		emit(res)
	}
	return res
}

// queryMerged streams results progressively as individual backends answer.
func (l *Launcher) queryMerged(ctx context.Context, text string, limit int, emit func(Result)) Result {
	ids := l.searchableIDs()
	if len(ids) == 0 {
		res := Result{}
		if emit != nil {
			emit(res)
		}
		return res
	}

	items := make([][]backend.Item, len(ids))
	errs := make([]error, len(ids))
	done := make([]bool, len(ids))
	var mu sync.Mutex

	var wg sync.WaitGroup
	var lastResult Result

	for i, id := range ids {
		wg.Add(1)
		go func(i int, id string) {
			defer wg.Done()
			found, err := l.queryBackend(ctx, id, text, limit, true, nil)

			mu.Lock()
			items[i] = found
			errs[i] = err
			done[i] = true

			groups := make([]rank.Group, 0, len(ids))
			completed := 0
			for j, jid := range ids {
				if done[j] {
					completed++
					if errs[j] == nil && len(items[j]) > 0 {
						groups = append(groups, rank.Group{Source: jid, Items: items[j]})
					}
				}
			}
			isFinal := (completed == len(ids))
			matches := rank.Rank(text, groups, nil, l.opts.RankOpts)
			res := Result{
				items:   flattenGroups(groups),
				matches: matches,
				partial: !isFinal,
			}
			lastResult = res
			mu.Unlock()

			if ctx.Err() == nil && emit != nil {
				if len(matches) > 0 || isFinal {
					emit(res)
				}
			}
		}(i, id)
	}
	wg.Wait()
	return lastResult
}

// queryBackend queries one backend with its own span and duration metric.
func (l *Launcher) queryBackend(ctx context.Context, id, text string, limit int, merged bool, scope []string) ([]backend.Item, error) {
	b, err := l.provider.Get(id)
	if err != nil {
		return nil, err
	}
	ctx, span := telemetry.StartSpan(ctx, "keyshell.backend.query",
		trace.WithAttributes(
			attribute.String(telemetry.Attrs.Backend, id),
			attribute.String("keyshell.query_text", text),
		))
	defer span.End()

	start := time.Now()
	stream, err := b.Query(ctx, backend.Query{Text: text, Limit: limit, Merged: merged, Scope: scope})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	items := drain(stream)
	stream.Close()
	telemetry.BackendQueryDuration().Record(ctx, time.Since(start).Seconds(),
		metric.WithAttributes(attribute.String(telemetry.Attrs.Backend, id)))
	return items, nil
}
