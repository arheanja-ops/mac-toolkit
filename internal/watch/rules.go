package watch

import (
	"fmt"
	"time"

	"github.com/arheanja-ops/mac-toolkit/internal/monitor"
)

// Alert severities, mildest first.
const (
	SevInfo     = "info"
	SevWarn     = "warn"
	SevCritical = "critical"
)

var sevRank = map[string]int{SevInfo: 1, SevWarn: 2, SevCritical: 3}

// Thresholds. A rule fires only when its condition holds for its whole window,
// so a short peak never alerts; they are constants on purpose (see the spec).
const (
	// Swap above 90% leaves almost no room before macOS must compress or kill.
	swapFullPct = 90.0
	// 20 MB/s of sustained swap-out is the machine writing RAM to disk all the
	// time: the classic "everything freezes" symptom.
	pagingMBps = 20.0
	// Load per core above 2 means twice as many runnable tasks as cores.
	loadPerCoreHigh = 2.0
	// With that load, idle below 20% is a busy CPU; above 50% the tasks are
	// waiting on memory or disk instead (same split as monitor.ClassifyLoad).
	idleBusyPct = 20.0
	idleFreePct = 50.0
	// Free disk below 10 GB or 5% makes swap and updates fail.
	diskLowGB  = 10.0
	diskLowPct = 5.0
)

// Rule is one sustained-threshold condition. Check returns whether the sample
// meets it, the severity and a short human value ("swap al 94 %").
type Rule struct {
	ID     string
	Title  string
	Action string
	Window time.Duration
	Check  func(Sample) (hit bool, sev, value string)
}

// Rules are the alert rules of the listening mode, in display order.
var Rules = []Rule{
	{ID: "memory_pressure", Title: "Presión de memoria", Window: 2 * time.Minute,
		Action: "Cierra apps que no uses.",
		Check: func(s Sample) (bool, string, string) {
			switch s.Pressure {
			case monitor.PressureCritical:
				return true, SevCritical, "presión crítica"
			case monitor.PressureWarning:
				return true, SevWarn, "presión en aviso"
			}
			return false, "", ""
		}},
	{ID: "swap_full", Title: "Swap casi lleno", Window: 2 * time.Minute,
		Action: "Cierra apps que no uses o reinicia cuando puedas.",
		Check: func(s Sample) (bool, string, string) {
			return s.SwapOK && s.SwapUsedPct > swapFullPct, SevWarn, fmt.Sprintf("swap al %.0f %%", s.SwapUsedPct)
		}},
	{ID: "paging", Title: "Paginación intensa", Window: 2 * time.Minute,
		Action: "La RAM no alcanza: cierra la app que más memoria usa.",
		Check: func(s Sample) (bool, string, string) {
			return s.RateOK && s.SwapOutMBps > pagingMBps, SevCritical, fmt.Sprintf("swap-out %.0f MB/s", s.SwapOutMBps)
		}},
	{ID: "cpu_saturated", Title: "CPU saturada", Window: 3 * time.Minute,
		Action: "Revisa la app que más CPU usa.",
		Check: func(s Sample) (bool, string, string) {
			lpc := s.LoadPerCore()
			return lpc > loadPerCoreHigh && s.CPUOK && s.IdlePct < idleBusyPct, SevWarn,
				fmt.Sprintf("carga %.1f por núcleo, libre %.0f %%", lpc, s.IdlePct)
		}},
	{ID: "load_waiting", Title: "Carga con CPU libre", Window: 3 * time.Minute,
		Action: "Procesos esperando memoria o disco: revisa Memory.",
		Check: func(s Sample) (bool, string, string) {
			lpc := s.LoadPerCore()
			return lpc > loadPerCoreHigh && s.CPUOK && s.IdlePct > idleFreePct, SevInfo,
				fmt.Sprintf("carga %.1f por núcleo, libre %.0f %%", lpc, s.IdlePct)
		}},
	{ID: "disk_low", Title: "Disco casi lleno", Window: 0,
		Action: "Libera espacio con Preview y Select & clean.",
		Check: func(s Sample) (bool, string, string) {
			return s.DiskOK && (s.DiskFreeGB < diskLowGB || s.DiskFreePct < diskLowPct), SevWarn,
				fmt.Sprintf("%.1f GB libres", s.DiskFreeGB)
		}},
	{ID: "thermal", Title: "Estado térmico", Window: time.Minute,
		Action: "Deja respirar el equipo y cierra tareas pesadas.",
		Check: func(s Sample) (bool, string, string) {
			return monitor.IsThermalAlert(s.Thermal), SevWarn, "térmico " + s.Thermal
		}},
}

// Alert states.
const (
	StateActive   = "active"
	StateResolved = "resolved"
)

// Alert is one rule that is (or was) firing.
type Alert struct {
	Rule     string    `json:"rule"`
	Title    string    `json:"title"`
	Severity string    `json:"severity"`
	Since    time.Time `json:"since"`
	Value    string    `json:"value"`
	Action   string    `json:"action,omitempty"`
	State    string    `json:"state"`
	Notified bool      `json:"notified,omitempty"`
}

// Event kinds.
const (
	EventAlert    = "alert"
	EventEscalate = "escalate"
	EventResolve  = "resolve"
)

// Event is a change of an alert: it fired, got worse, or resolved.
type Event struct {
	Time  time.Time `json:"t"`
	Kind  string    `json:"kind"`
	Alert Alert     `json:"alert"`
}

type ruleState struct {
	condSince  time.Time // first sample of the current run meeting the rule
	clearSince time.Time // first sample of the current run NOT meeting it
	active     *Alert
}

// Evaluator applies the rules to a stream of samples. Samples further apart
// than MaxGap (a pause, a sleeping Mac) break continuity: a window never spans
// a gap the toolkit did not observe.
type Evaluator struct {
	Rules  []Rule
	MaxGap time.Duration
	state  map[string]*ruleState
	last   time.Time
}

// NewEvaluator uses the default rules.
func NewEvaluator() *Evaluator {
	return &Evaluator{Rules: Rules, MaxGap: 3 * DefaultInterval, state: map[string]*ruleState{}}
}

// Feed evaluates one sample and returns the events it produced.
func (e *Evaluator) Feed(s Sample) []Event {
	if e.state == nil {
		e.state = map[string]*ruleState{}
	}
	gap := !e.last.IsZero() && e.MaxGap > 0 && s.Time.Sub(e.last) > e.MaxGap
	e.last = s.Time
	var evs []Event
	for _, r := range e.Rules {
		st := e.state[r.ID]
		if st == nil {
			st = &ruleState{}
			e.state[r.ID] = st
		}
		if gap {
			st.condSince, st.clearSince = time.Time{}, time.Time{}
		}
		hit, sev, value := r.Check(s)
		if hit {
			evs = append(evs, e.onHit(r, st, s.Time, sev, value)...)
		} else {
			evs = append(evs, e.onClear(r, st, s.Time)...)
		}
	}
	return evs
}

func (e *Evaluator) onHit(r Rule, st *ruleState, t time.Time, sev, value string) []Event {
	st.clearSince = time.Time{}
	if st.condSince.IsZero() {
		st.condSince = t
	}
	if st.active == nil {
		if t.Sub(st.condSince) < r.Window {
			return nil
		}
		st.active = &Alert{Rule: r.ID, Title: r.Title, Severity: sev, Since: st.condSince,
			Value: value, Action: r.Action, State: StateActive}
		return []Event{{Time: t, Kind: EventAlert, Alert: *st.active}}
	}
	st.active.Value = value
	if sevRank[sev] > sevRank[st.active.Severity] {
		st.active.Severity = sev
		return []Event{{Time: t, Kind: EventEscalate, Alert: *st.active}}
	}
	return nil
}

// onClear resolves an active alert once the condition stayed clear for the
// rule's window (at least one sample for immediate rules).
func (e *Evaluator) onClear(r Rule, st *ruleState, t time.Time) []Event {
	st.condSince = time.Time{}
	if st.active == nil {
		return nil
	}
	if st.clearSince.IsZero() {
		st.clearSince = t
	}
	if t.Sub(st.clearSince) < r.Window {
		return nil
	}
	a := *st.active
	a.State = StateResolved
	st.active, st.clearSince = nil, time.Time{}
	return []Event{{Time: t, Kind: EventResolve, Alert: a}}
}

// Active lists the firing alerts in rule order.
func (e *Evaluator) Active() []Alert {
	var out []Alert
	for _, r := range e.Rules {
		if st := e.state[r.ID]; st != nil && st.active != nil {
			out = append(out, *st.active)
		}
	}
	return out
}

// MarkNotified records that the active alert of a rule was notified.
func (e *Evaluator) MarkNotified(rule string) {
	if st := e.state[rule]; st != nil && st.active != nil {
		st.active.Notified = true
	}
}
