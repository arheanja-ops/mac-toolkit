package watch

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/arheanja-ops/mac-toolkit/internal/monitor"
)

var t0 = time.Date(2026, 1, 2, 8, 0, 0, 0, time.Local)

// series feeds n samples 15 s apart, built by mk(i).
func series(e *Evaluator, start time.Time, n int, mk func(i int) Sample) []Event {
	var evs []Event
	for i := 0; i < n; i++ {
		s := mk(i)
		s.Time = start.Add(time.Duration(i) * DefaultInterval)
		evs = append(evs, e.Feed(s)...)
	}
	return evs
}

func swapAt(pct float64) func(int) Sample {
	return func(int) Sample { return Sample{SwapOK: true, SwapUsedPct: pct} }
}

func kinds(evs []Event) string {
	var k []string
	for _, e := range evs {
		k = append(k, e.Alert.Rule+":"+e.Kind)
	}
	return strings.Join(k, ",")
}

func TestShortPeakDoesNotAlert(t *testing.T) {
	e := NewEvaluator()
	evs := series(e, t0, 4, swapAt(95)) // 45 s < 2 min
	evs = append(evs, series(e, t0.Add(time.Minute), 4, swapAt(50))...)
	if len(evs) != 0 || len(e.Active()) != 0 {
		t.Fatalf("a short peak must not alert: %s", kinds(evs))
	}
}

func TestSustainedAlertsThenResolves(t *testing.T) {
	e := NewEvaluator()
	evs := series(e, t0, 9, swapAt(95)) // 0..120 s
	if kinds(evs) != "swap_full:alert" || len(e.Active()) != 1 {
		t.Fatalf("sustained swap must alert once: %s", kinds(evs))
	}
	if a := e.Active()[0]; !a.Since.Equal(t0) || a.Value != "swap al 95 %" {
		t.Fatalf("alert: %+v", a)
	}
	evs = series(e, t0.Add(135*time.Second), 9, swapAt(40))
	if kinds(evs) != "swap_full:resolve" || len(e.Active()) != 0 {
		t.Fatalf("recovery must resolve: %s", kinds(evs))
	}
}

func TestEscalationAndGap(t *testing.T) {
	e := NewEvaluator()
	pressure := func(p string) func(int) Sample { return func(int) Sample { return Sample{Pressure: p} } }
	evs := series(e, t0, 9, pressure(monitor.PressureWarning))
	evs = append(evs, series(e, t0.Add(135*time.Second), 1, pressure(monitor.PressureCritical))...)
	if kinds(evs) != "memory_pressure:alert,memory_pressure:escalate" || evs[1].Alert.Severity != SevCritical {
		t.Fatalf("escalation: %s", kinds(evs))
	}
	// A gap (paused or asleep) breaks the window of a new condition.
	f := NewEvaluator()
	series(f, t0, 5, swapAt(95))
	evs = series(f, t0.Add(30*time.Minute), 5, swapAt(95))
	if len(evs) != 0 {
		t.Fatalf("a window must not span a gap: %s", kinds(evs))
	}
}

func TestEveryRuleCanFire(t *testing.T) {
	cases := map[string]Sample{
		"paging":        {RateOK: true, SwapOutMBps: 30},
		"cpu_saturated": {LoadOK: true, Load5: 30, Cores: 10, CPUOK: true, IdlePct: 5},
		"load_waiting":  {LoadOK: true, Load5: 30, Cores: 10, CPUOK: true, IdlePct: 80},
		"disk_low":      {DiskOK: true, DiskFreeGB: 4, DiskFreePct: 20},
		"thermal":       {Thermal: monitor.ThermalThrottled},
	}
	for rule, s := range cases {
		e := NewEvaluator()
		evs := series(e, t0, 13, func(int) Sample { return s })
		if len(evs) != 1 || evs[0].Alert.Rule != rule {
			t.Fatalf("%s: %s", rule, kinds(evs))
		}
	}
	// Unknown readings never alert.
	e := NewEvaluator()
	if evs := series(e, t0, 20, func(int) Sample { return Sample{Pressure: monitor.PressureUnknown, Thermal: monitor.ThermalUnknown} }); len(evs) != 0 {
		t.Fatalf("unknown must stay quiet: %s", kinds(evs))
	}
}

type fakeRun struct{ calls [][]string }

func (f *fakeRun) run(name string, args ...string) error {
	f.calls = append(f.calls, append([]string{name}, args...))
	return nil
}

func ev(kind, rule, sev string, at time.Time) Event {
	return Event{Time: at, Kind: kind, Alert: Alert{Rule: rule, Title: rule, Severity: sev, Since: at, Value: "x"}}
}

func TestNotifierCooldownEscalationAndCap(t *testing.T) {
	f := &fakeRun{}
	n := &Notifier{Run: f.run, Enabled: true, Cooldown: ruleCooldown, PerHour: notifyPerHour}
	if !n.Handle(ev(EventAlert, "swap_full", SevWarn, t0)) {
		t.Fatal("first alert must notify")
	}
	if n.Handle(ev(EventAlert, "swap_full", SevWarn, t0.Add(5*time.Minute))) {
		t.Fatal("cooldown must suppress the repeat")
	}
	if !n.Handle(ev(EventEscalate, "swap_full", SevCritical, t0.Add(6*time.Minute))) {
		t.Fatal("escalation must bypass the cooldown")
	}
	if !n.Handle(ev(EventAlert, "swap_full", SevWarn, t0.Add(25*time.Minute))) {
		t.Fatal("after the cooldown it notifies again")
	}
	if n.Handle(ev(EventResolve, "disk_low", SevWarn, t0.Add(26*time.Minute))) {
		t.Fatal("resolved warn must not notify")
	}
	if !n.Handle(ev(EventResolve, "paging", SevCritical, t0.Add(27*time.Minute))) {
		t.Fatal("resolved critical notifies")
	}
	// 4 sent so far within the hour; 2 more fit, the 7th is capped.
	n.Handle(ev(EventAlert, "a", SevWarn, t0.Add(28*time.Minute)))
	n.Handle(ev(EventAlert, "b", SevWarn, t0.Add(29*time.Minute)))
	if n.Handle(ev(EventAlert, "c", SevWarn, t0.Add(30*time.Minute))) {
		t.Fatal("hourly cap of 6 must hold")
	}
	if len(f.calls) != 6 {
		t.Fatalf("osascript calls: %d", len(f.calls))
	}
	for _, c := range f.calls {
		if c[0] != "osascript" || c[1] != "-e" || len(c) != 3 {
			t.Fatalf("must call osascript -e <script> without a shell: %v", c)
		}
	}
}

func TestDisabledNotifierNeverRuns(t *testing.T) {
	f := &fakeRun{}
	n := &Notifier{Run: f.run, Enabled: false, Cooldown: ruleCooldown, PerHour: notifyPerHour}
	if n.Handle(ev(EventAlert, "paging", SevCritical, t0)) || len(f.calls) != 0 {
		t.Fatal("--no-notify must never invoke osascript")
	}
}

func TestNotificationTextIsEscaped(t *testing.T) {
	got := script(`Sub "x"`, "line1\nline2 \"quoted\" back\\slash")
	want := `display notification "line1 line2 \"quoted\" back\\slash" with title "mac-toolkit" subtitle "Sub \"x\""`
	if got != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
	_, text := message(Event{Time: t0.Add(2 * time.Minute), Kind: EventAlert,
		Alert: Alert{Title: "Swap casi lleno", Value: "swap al 94 %", Action: "Cierra apps que no uses.", Since: t0}})
	if text != "Swap al 94 % hace 2 min. Cierra apps que no uses." {
		t.Fatalf("text: %q", text)
	}
}

func TestStoreWritesReadsAndPrunesOnlyItsFiles(t *testing.T) {
	dir := t.TempDir()
	st := Store{Dir: dir}
	s := Sample{Time: t0, MemOK: true, MemUsedPct: 70, TopCPUApp: "Builder"}
	e := ev(EventAlert, "swap_full", SevWarn, t0.Add(time.Minute))
	for _, r := range []Record{{Type: RecordSample, Sample: &s}, {Type: RecordEvent, Event: &e}} {
		if err := st.Append(r); err != nil {
			t.Fatal(err)
		}
	}
	recs, err := st.Read(t0.Add(-time.Hour), t0.Add(time.Hour))
	if err != nil || len(recs) != 2 || recs[0].Sample.TopCPUApp != "Builder" || recs[1].Event.Alert.Rule != "swap_full" {
		t.Fatalf("read: %v %+v", err, recs)
	}
	info, _ := os.Stat(filepath.Join(dir, "2026-01-02.jsonl"))
	if info == nil || info.Mode().Perm() != 0o600 {
		t.Fatal("history file must exist and be private")
	}
	// Retention: old jsonl goes; recent jsonl, other files and folders stay.
	keep := []string{"2026-01-02.jsonl", "2025-12-30.jsonl", "notes.txt", "2025-01-01.json", "2025-01-01.jsonl.bak"}
	for _, n := range append(keep[1:], "2025-12-01.jsonl") {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("{}\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "2025-01-02.jsonl"), 0o700); err != nil {
		t.Fatal(err)
	}
	removed, err := st.Prune(t0)
	if err != nil || strings.Join(removed, ",") != "2025-12-01.jsonl" {
		t.Fatalf("prune removed %v (%v)", removed, err)
	}
	for _, n := range append(keep, "2025-01-02.jsonl") {
		if _, err := os.Stat(filepath.Join(dir, n)); err != nil {
			t.Fatalf("%s must survive: %v", n, err)
		}
	}
}

func TestHistoryHasNoPathsOrPIDs(t *testing.T) {
	s := Sample{Time: t0, TopCPUApp: "Builder", TopMemApp: "Editor"}
	b, _ := json.Marshal(Record{Type: RecordSample, Sample: &s})
	if strings.ContainsAny(string(b), "/~") || strings.Contains(string(b), "pid") {
		t.Fatalf("history line leaks: %s", b)
	}
}

func TestSummarizeRange(t *testing.T) {
	var recs []Record
	for i := 0; i < 20; i++ {
		s := Sample{Time: t0.Add(time.Duration(i) * DefaultInterval), MemOK: true, MemUsedPct: float64(50 + i),
			LoadOK: true, Load5: 10, Cores: 10, TopCPUApp: "Builder", TopMemApp: "Editor"}
		if i%4 == 0 {
			s.TopCPUApp = "Other"
		}
		recs = append(recs, Record{Type: RecordSample, Sample: &s})
	}
	a := ev(EventAlert, "swap_full", SevWarn, t0.Add(2*time.Minute))
	r := ev(EventResolve, "swap_full", SevWarn, t0.Add(4*time.Minute))
	old := ev(EventResolve, "paging", SevCritical, t0.Add(time.Minute))
	recs = append(recs, Record{Type: RecordEvent, Event: &old}, Record{Type: RecordEvent, Event: &a}, Record{Type: RecordEvent, Event: &r})
	sum := Summarize(recs, t0, t0.Add(time.Hour))
	if sum.Samples != 20 || sum.MemUsedPct.Min != 50 || sum.MemUsedPct.Max != 69 || sum.MemUsedPct.Avg != 59.5 || sum.MemUsedPct.P95 != 68 {
		t.Fatalf("mem stat: %+v", sum.MemUsedPct)
	}
	if sum.LoadPerCore.Max != 1 || sum.TopCPU[0].Name != "Builder" || sum.TopCPU[0].Count != 15 {
		t.Fatalf("load/top: %+v %+v", sum.LoadPerCore, sum.TopCPU)
	}
	if len(sum.Alerts) != 2 || sum.Alerts[1].End.IsZero() || sum.Alerts[0].Rule != "paging" {
		t.Fatalf("alerts: %+v", sum.Alerts)
	}
	if rows := Hourly(recs); len(rows) != 1 || rows[0].Samples != 20 || rows[0].AlertsFired != 1 {
		t.Fatalf("hourly: %+v", rows)
	}
}

func fakeReading(swapout int64) func(monitor.LightOptions) monitor.LightReading {
	return func(opt monitor.LightOptions) monitor.LightReading {
		r := monitor.LightReading{MemOK: true, MemUsedPct: 60, SwapOK: true, SwapUsedMB: 950, SwapTotalMB: 1000,
			SwapoutBytes: swapout, LoadOK: true, Load5: 2, Cores: 8, PressureCode: 1}
		if opt.Thermal {
			r.Thermal = monitor.ThermalNormal
		}
		if opt.Apps {
			r.TopCPUApp = "Builder"
		}
		return r
	}
}

func TestSamplerDerivesRatesAndKeepsThermal(t *testing.T) {
	var out int64
	sm := &Sampler{Read: func(o monitor.LightOptions) monitor.LightReading { return fakeReading(out)(o) },
		DiskFree: func() (int64, int64, bool) { return 50 << 30, 500 << 30, true }}
	a := sm.Take(t0)
	out = 300 * mib
	b := sm.Take(t0.Add(15 * time.Second))
	if a.RateOK || !b.RateOK || b.SwapOutMBps != 20 {
		t.Fatalf("rate: %+v", b)
	}
	if b.TopCPUApp != "Builder" {
		t.Fatal("top app is kept between ps reads")
	}
	if b.Thermal != monitor.ThermalNormal || b.SwapUsedPct != 95 || b.DiskFreeGB != 50 || b.DiskFreePct != 10 {
		t.Fatalf("sample: %+v", b)
	}
}

func TestEngineStepRecordsAndNotifies(t *testing.T) {
	dir := t.TempDir()
	f := &fakeRun{}
	now := t0
	var printed []Event
	eng := &Engine{Interval: DefaultInterval, Eval: NewEvaluator(), Store: &Store{Dir: dir},
		Sampler:  &Sampler{Read: fakeReading(0)},
		Notifier: &Notifier{Run: f.run, Enabled: true, Cooldown: ruleCooldown, PerHour: notifyPerHour},
		Now:      func() time.Time { return now }, OnEvent: func(e Event) { printed = append(printed, e) }}
	for i := 0; i < 9; i++ {
		eng.Step()
		now = now.Add(DefaultInterval)
	}
	st := eng.Status()
	if len(st.Active) != 1 || !st.Active[0].Notified || len(f.calls) != 1 || len(printed) != 1 {
		t.Fatalf("status %+v calls %d printed %d", st, len(f.calls), len(printed))
	}
	if on := eng.ToggleNotify(); on || eng.Status().Notify {
		t.Fatal("toggle notify must turn notifications off")
	}
	if !eng.TogglePause() || !eng.Status().Paused || eng.Status().Listening() {
		t.Fatal("toggle pause")
	}
	recs, _ := eng.Store.Read(t0, now)
	if len(recs) != 10 { // 9 samples + 1 event
		t.Fatalf("records: %d", len(recs))
	}
}

func BenchmarkSamplingCycle(b *testing.B) {
	eng := &Engine{Interval: DefaultInterval, Eval: NewEvaluator(), Store: &Store{Dir: b.TempDir()},
		Sampler: &Sampler{Read: fakeReading(0)}, Notifier: &Notifier{}, Now: time.Now}
	for i := 0; i < b.N; i++ {
		eng.Step()
	}
}
