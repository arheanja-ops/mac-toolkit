package monitor

// Monitor is the interface all system monitors implement
type Monitor interface {
	Name() string
	Snapshot() (map[string]any, error)
	Display() error
}
