package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/arheanja-ops/mac-toolkit/internal/core"
	"github.com/arheanja-ops/mac-toolkit/internal/watch"

	"github.com/spf13/cobra"
)

var watchNoNotify bool

// watchSummaryEvery is how often `watch` prints a one-line status.
const watchSummaryEvery = 15 * time.Minute

var watchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Listen to the system and alert on sustained problems (read-only)",
	Long: "Samples the machine every 15 s with light tools, alerts when a problem holds " +
		"(memory pressure, swap, paging, CPU, disk, thermal) and keeps a 7-day history. " +
		"It never changes anything. Stop it with Ctrl+C.",
	Run: func(cmd *cobra.Command, args []string) {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		eng := watch.NewEngine(!watchNoNotify)
		eng.OnEvent = func(e watch.Event) { fmt.Println(watch.EventLine(e)) }
		notify := "on"
		if watchNoNotify {
			notify = "off"
		}
		core.Info("Escuchando cada %s · notificaciones %s · historial en %s", watch.DefaultInterval, notify, core.HistoryDir())
		core.Info("Ctrl+C para salir")
		go func() {
			t := time.NewTicker(watchSummaryEvery)
			defer t.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-t.C:
					fmt.Println(watch.StatusLine(eng.Status()))
				}
			}
		}()
		eng.Run(ctx)
		core.Success("Escucha detenida")
	},
}

func init() {
	watchCmd.Flags().BoolVar(&watchNoNotify, "no-notify", false, "do not show macOS notifications")
	rootCmd.AddCommand(watchCmd)
}
