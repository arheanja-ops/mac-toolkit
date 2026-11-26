package cmd

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/arheanja-ops/mac-toolkit/internal/core"
	"github.com/arheanja-ops/mac-toolkit/internal/watch"

	"github.com/spf13/cobra"
)

var (
	historySince string
	historyUntil string
	historyJSON  bool
)

var historyCmd = &cobra.Command{
	Use:   "history",
	Short: "Summarize what the listening mode recorded in a time range",
	Long: "Summary of the watch history (default: last 24 h): min/avg/p95/max of memory, " +
		"swap, paging, load per core, idle CPU and free disk, the alerts with start and end, " +
		"and the apps that were most often on top. --since/--until take HH:MM (today) or " +
		"YYYY-MM-DDTHH:MM.",
	RunE: func(cmd *cobra.Command, args []string) error {
		now := time.Now()
		from, to := now.Add(-24*time.Hour), now
		var err error
		if historySince != "" {
			if from, err = parseWhen(historySince, now); err != nil {
				return err
			}
		}
		if historyUntil != "" {
			if to, err = parseWhen(historyUntil, now); err != nil {
				return err
			}
		}
		if !from.Before(to) {
			return fmt.Errorf("--since must be before --until")
		}
		recs, err := watch.Store{Dir: core.HistoryDir()}.Read(from, to)
		if err != nil {
			return err
		}
		sum := watch.Summarize(recs, from, to)
		if historyJSON {
			b, err := json.MarshalIndent(sum, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(b))
			return nil
		}
		fmt.Print(watch.FormatSummary(sum))
		return nil
	},
}

// parseWhen accepts "HH:MM" (today, local time) or "YYYY-MM-DDTHH:MM".
func parseWhen(s string, now time.Time) (time.Time, error) {
	s = strings.TrimSpace(s)
	if t, err := time.ParseInLocation("2006-01-02T15:04", s, time.Local); err == nil {
		return t, nil
	}
	if t, err := time.ParseInLocation("15:04", s, time.Local); err == nil {
		return time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, time.Local), nil
	}
	return time.Time{}, fmt.Errorf("invalid time %q: use HH:MM or YYYY-MM-DDTHH:MM", s)
}

func init() {
	historyCmd.Flags().StringVar(&historySince, "since", "", "start (HH:MM or YYYY-MM-DDTHH:MM); default 24 h ago")
	historyCmd.Flags().StringVar(&historyUntil, "until", "", "end (HH:MM or YYYY-MM-DDTHH:MM); default now")
	historyCmd.Flags().BoolVar(&historyJSON, "json", false, "print the summary as JSON")
	rootCmd.AddCommand(historyCmd)
}
