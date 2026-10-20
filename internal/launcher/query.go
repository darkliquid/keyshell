package launcher

import (
	"context"

	"github.com/darkliquid/keyshell/internal/backend"
	"github.com/darkliquid/keyshell/internal/rank"
)

// Job is one search request. It carries the whole state the query needs, so the
// worker never touches launcher state while it runs.
type Job struct {
	gen   uint64
	query string
	mode  string
	scope []string
	depth int
}

// Result is one completed search, tagged with the generation it answers. A
// result whose generation is no longer current is a superseded keystroke and
// is dropped, so a slow plugin can never publish stale results.
type Result struct {
	gen     uint64
	depth   int
	items   []backend.Item
	matches []rank.Match
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
	l.resultsCh = make(chan Result, 1)
	go l.work(l.jobs, l.resultsCh)
}

// work runs queries off the UI thread. Only the newest request matters: a
// pending one is replaced rather than queued, because a merged search costs a
// plugin round trip and queuing keystrokes would make the results lag behind
// the field.
func (l *Launcher) work(jobs <-chan Job, out chan Result) {
	for job := range jobs {
		res := l.run(job)
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
		if l.apply(l.run(job)) {
			l.changed()
		}
		return
	}
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
	level.Items = res.items
	level.Matches = res.matches
	l.results = res.matches
	return true
}

// jobLocked snapshots the state one query needs and bumps the generation.
func (l *Launcher) jobLocked() Job {
	l.gen++
	var scope []string
	if level := l.currentLevelLocked(); level != nil {
		scope = append([]string(nil), level.Scope...)
	}
	return Job{
		gen:   l.gen,
		query: l.query,
		mode:  l.mode,
		scope: scope,
		depth: len(l.levels),
	}
}

// run executes one query against the provider. It touches no launcher state, so
// it is safe off the UI thread.
func (l *Launcher) run(job Job) Result {
	ctx := context.Background()
	res := Result{gen: job.gen, depth: job.depth}
	if job.mode == MergedMode || job.mode == "" {
		groups := []rank.Group{}
		for _, id := range l.searchableIDs() {
			b, err := l.provider.Get(id)
			if err != nil {
				continue
			}
			stream, err := b.Query(ctx, backend.Query{Text: job.query, Limit: l.opts.RankOpts.Limit, Merged: true})
			if err != nil {
				continue
			}
			items := drain(stream)
			stream.Close()
			groups = append(groups, rank.Group{Source: id, Items: items})
		}
		res.items = flattenGroups(groups)
		res.matches = rank.Rank(job.query, groups, nil, l.opts.RankOpts)
		return res
	}
	b, err := l.provider.Get(job.mode)
	if err != nil {
		return res
	}
	stream, err := b.Query(ctx, backend.Query{Text: job.query, Limit: l.opts.RankOpts.Limit, Merged: false, Scope: job.scope})
	if err != nil {
		return res
	}
	res.items = drain(stream)
	stream.Close()
	res.matches = rank.RankOne(job.query, job.mode, res.items, nil, l.opts.RankOpts)
	return res
}
