package watch

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

// Retention of the history files.
const historyKeep = 7 * 24 * time.Hour

// Record types of a history line.
const (
	RecordSample = "sample"
	RecordEvent  = "event"
)

// Record is one line of a daily history file: a sample or an alert event.
type Record struct {
	Type   string  `json:"type"`
	Sample *Sample `json:"sample,omitempty"`
	Event  *Event  `json:"event,omitempty"`
}

// Time is the moment the record describes.
func (r Record) Time() time.Time {
	switch {
	case r.Sample != nil:
		return r.Sample.Time
	case r.Event != nil:
		return r.Event.Time
	}
	return time.Time{}
}

// reHistoryFile matches the only files the store writes, and so the only ones
// Prune may ever delete.
var reHistoryFile = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2})\.jsonl$`)

const dayLayout = "2006-01-02"

// Store is the daily JSONL history in Dir (one file per local day).
type Store struct{ Dir string }

func (s Store) file(day time.Time) string {
	return filepath.Join(s.Dir, day.Format(dayLayout)+".jsonl")
}

// Append writes one record to the file of its day. Files are private (0600).
func (s Store) Append(r Record) error {
	if err := os.MkdirAll(s.Dir, 0o700); err != nil {
		return fmt.Errorf("creating history dir: %w", err)
	}
	line, err := json.Marshal(r)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(s.file(r.Time().Local()), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("opening history: %w", err)
	}
	defer f.Close()
	_, err = f.Write(append(line, '\n'))
	return err
}

// Read returns the records with since <= time < until, oldest first. Missing
// days and unparsable lines are skipped.
func (s Store) Read(since, until time.Time) ([]Record, error) {
	var out []Record
	day := time.Date(since.Year(), since.Month(), since.Day(), 0, 0, 0, 0, time.Local)
	for ; day.Before(until); day = day.AddDate(0, 0, 1) {
		f, err := os.Open(s.file(day))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return out, err
		}
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 64*1024), 1024*1024)
		for sc.Scan() {
			var r Record
			if json.Unmarshal(sc.Bytes(), &r) != nil {
				continue
			}
			if t := r.Time(); !t.Before(since) && t.Before(until) {
				out = append(out, r)
			}
		}
		f.Close()
	}
	return out, nil
}

// Prune deletes the history files older than the retention. It only touches
// regular files named YYYY-MM-DD.jsonl directly inside Dir; anything else is
// left alone. It returns the names it removed.
func (s Store) Prune(now time.Time) ([]string, error) {
	entries, err := os.ReadDir(s.Dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	cutoff := now.Add(-historyKeep)
	var removed []string
	for _, e := range entries {
		m := reHistoryFile.FindStringSubmatch(e.Name())
		if m == nil || !e.Type().IsRegular() {
			continue
		}
		day, err := time.ParseInLocation(dayLayout, m[1], time.Local)
		if err != nil || !day.AddDate(0, 0, 1).Before(cutoff) {
			continue
		}
		if err := os.Remove(filepath.Join(s.Dir, e.Name())); err == nil {
			removed = append(removed, e.Name())
		}
	}
	return removed, nil
}
