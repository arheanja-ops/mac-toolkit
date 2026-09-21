package tui

import "errors"

// errBoom is a sentinel error used across tests to exercise failure paths.
var errBoom = errors.New("boom")

// fakeMonitor implements monitor.Monitor with injectable snapshot data/error,
// so adapter tests never touch real hardware commands.
type fakeMonitor struct {
	name string
	data map[string]any
	err  error
}

func (f fakeMonitor) Name() string                     { return f.name }
func (f fakeMonitor) Snapshot() (map[string]any, error) { return f.data, f.err }
func (f fakeMonitor) Display() error                    { return f.err }
