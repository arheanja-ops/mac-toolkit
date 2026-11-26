package watch

import (
	"context"
	"sync"
	"time"

	"github.com/arheanja-ops/mac-toolkit/internal/core"
)

// Engine runs the sampler on a ticker, feeds the rules, writes the history and
// notifies. It is safe for concurrent use: the TUI reads Status while the loop
// runs in its own goroutine.
type Engine struct {
	Interval time.Duration
	Sampler  *Sampler
	Eval     *Evaluator
	Notifier *Notifier
	Store    *Store
	Now      func() time.Time
	// OnEvent is called for every alert event (the CLI prints them).
	OnEvent func(Event)

	mu      sync.Mutex
	last    Sample
	hasLast bool
	running bool
	paused  bool
	errs    int
}

// NewEngine wires the engine to the real machine, the canonical history folder
// and osascript notifications (when notify is true).
func NewEngine(notify bool) *Engine {
	return &Engine{
		Interval: DefaultInterval, Sampler: NewSampler(), Eval: NewEvaluator(),
		Notifier: NewNotifier(notify), Store: &Store{Dir: core.HistoryDir()}, Now: time.Now,
	}
}

// Status is a consistent copy of the engine state.
type Status struct {
	Running   bool
	Paused    bool
	Notify    bool
	Last      Sample
	HasLast   bool
	Active    []Alert
	WriteErrs int
}

// Listening is true while the loop runs and is not paused.
func (s Status) Listening() bool { return s.Running && !s.Paused }

// Status returns the current state.
func (e *Engine) Status() Status {
	e.mu.Lock()
	defer e.mu.Unlock()
	return Status{Running: e.running, Paused: e.paused, Notify: e.Notifier != nil && e.Notifier.Enabled,
		Last: e.last, HasLast: e.hasLast, Active: e.Eval.Active(), WriteErrs: e.errs}
}

// TogglePause pauses or resumes the listening; it returns true when paused.
func (e *Engine) TogglePause() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.paused = !e.paused
	return e.paused
}

// ToggleNotify turns notifications on or off; it returns the new state.
func (e *Engine) ToggleNotify() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.Notifier == nil {
		return false
	}
	e.Notifier.Enabled = !e.Notifier.Enabled
	return e.Notifier.Enabled
}

// Step takes one sample and processes it. The read happens outside the lock
// (it takes about a second), so Status never blocks on the machine.
func (e *Engine) Step() []Event {
	s := e.Sampler.Take(e.Now())
	e.mu.Lock()
	defer e.mu.Unlock()
	e.last, e.hasLast = s, true
	evs := e.Eval.Feed(s)
	e.record(Record{Type: RecordSample, Sample: &s})
	for i := range evs {
		if e.Notifier.Handle(evs[i]) {
			evs[i].Alert.Notified = true
			if evs[i].Kind != EventResolve {
				e.Eval.MarkNotified(evs[i].Alert.Rule)
			}
		}
		ev := evs[i]
		e.record(Record{Type: RecordEvent, Event: &ev})
	}
	if e.OnEvent != nil {
		for _, ev := range evs {
			e.OnEvent(ev)
		}
	}
	return evs
}

func (e *Engine) record(r Record) {
	if e.Store == nil {
		return
	}
	if err := e.Store.Append(r); err != nil {
		e.errs++
	}
}

// Run prunes the old history, samples right away and then every Interval until
// ctx ends. While paused the ticker keeps running but nothing is read.
func (e *Engine) Run(ctx context.Context) {
	if e.Store != nil {
		_, _ = e.Store.Prune(e.Now())
	}
	e.mu.Lock()
	e.running = true
	e.mu.Unlock()
	defer func() {
		e.mu.Lock()
		e.running = false
		e.mu.Unlock()
	}()
	tick := time.NewTicker(e.Interval)
	defer tick.Stop()
	for {
		if !e.Status().Paused {
			e.Step()
		}
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}
