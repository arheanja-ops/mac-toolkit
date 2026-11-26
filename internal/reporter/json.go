package reporter

import (
	"encoding/json"
	"fmt"
	"os"
)

// JSON serializes the model itself (schema_version included), so the file is
// exactly what the markdown and HTML were rendered from.
func (r Report) JSON() ([]byte, error) {
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encoding json report: %w", err)
	}
	return append(b, '\n'), nil
}

// LoadJSON reads a report.json. A file written by a newer schema is refused
// instead of being half-understood.
func LoadJSON(path string) (Report, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Report{}, fmt.Errorf("reading report: %w", err)
	}
	var r Report
	if err := json.Unmarshal(data, &r); err != nil {
		return Report{}, fmt.Errorf("parsing report: %w", err)
	}
	if r.SchemaVersion < 1 || r.SchemaVersion > SchemaVersion {
		return Report{}, fmt.Errorf("unsupported report schema version %d", r.SchemaVersion)
	}
	return r, nil
}
