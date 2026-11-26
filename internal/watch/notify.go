package watch

import (
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// Anti-spam limits.
const (
	ruleCooldown  = 15 * time.Minute
	notifyPerHour = 6
)

// Notifier turns alert events into macOS notifications, with a per-rule
// cooldown (re-notified earlier only when the severity escalates), a global
// hourly cap and "resolved" notices only for critical alerts. Run is the
// injectable runner: tests replace it so nothing is ever shown.
type Notifier struct {
	Run      func(name string, args ...string) error
	Enabled  bool
	Cooldown time.Duration
	PerHour  int

	lastAt  map[string]time.Time
	lastSev map[string]string
	sent    []time.Time
}

// NewNotifier returns a notifier wired to osascript.
func NewNotifier(enabled bool) *Notifier {
	return &Notifier{
		Run:     func(name string, args ...string) error { return exec.Command(name, args...).Run() },
		Enabled: enabled, Cooldown: ruleCooldown, PerHour: notifyPerHour,
	}
}

// Handle decides whether ev deserves a notification and sends it. It reports
// whether a notification was sent. Disabled notifiers never call Run.
func (n *Notifier) Handle(ev Event) bool {
	if n == nil || !n.Enabled || n.Run == nil {
		return false
	}
	if n.lastAt == nil {
		n.lastAt, n.lastSev = map[string]time.Time{}, map[string]string{}
	}
	a := ev.Alert
	switch ev.Kind {
	case EventResolve:
		if a.Severity != SevCritical {
			return false
		}
	case EventAlert, EventEscalate:
		last, seen := n.lastAt[a.Rule]
		escalated := sevRank[a.Severity] > sevRank[n.lastSev[a.Rule]]
		if seen && ev.Time.Sub(last) < n.Cooldown && !escalated {
			return false
		}
	default:
		return false
	}
	if !n.underCap(ev.Time) {
		return false
	}
	title, text := message(ev)
	if err := n.Run("osascript", "-e", script(title, text)); err != nil {
		return false
	}
	n.sent = append(n.sent, ev.Time)
	if ev.Kind != EventResolve {
		n.lastAt[a.Rule], n.lastSev[a.Rule] = ev.Time, a.Severity
	}
	return true
}

// underCap drops sends older than an hour and checks the hourly cap.
func (n *Notifier) underCap(now time.Time) bool {
	kept := n.sent[:0]
	for _, t := range n.sent {
		if now.Sub(t) < time.Hour {
			kept = append(kept, t)
		}
	}
	n.sent = kept
	return n.PerHour <= 0 || len(n.sent) < n.PerHour
}

// message is the subtitle (rule) and the short, cautious text of a
// notification: the figure, how long, and the suggested action.
func message(ev Event) (subtitle, text string) {
	a := ev.Alert
	if ev.Kind == EventResolve {
		return a.Title, "Se normalizó: " + a.Value + "."
	}
	mins := int(ev.Time.Sub(a.Since).Minutes())
	since := "ahora"
	if mins >= 1 {
		since = "hace " + strconv.Itoa(mins) + " min"
	}
	return a.Title, capitalize(a.Value) + " " + since + ". " + a.Action
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	return strings.ToUpper(string(r[0])) + string(r[1:])
}

// script builds the AppleScript passed to osascript as ONE argument (no shell).
func script(subtitle, text string) string {
	return `display notification "` + escapeAS(text) + `" with title "mac-toolkit" subtitle "` + escapeAS(subtitle) + `"`
}

// escapeAS makes a string safe inside an AppleScript string literal: control
// characters become spaces and backslashes and quotes are escaped.
func escapeAS(s string) string {
	s = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return ' '
		}
		return r
	}, s)
	return strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s)
}
