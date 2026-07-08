package mainlogic

import (
	"log/slog"
	"time"
)

func poolStats() {
	for {
		time.Sleep(120 * time.Second)
		for _, pool := range pools.Items() {
			pool.PrintStats(slog.LevelInfo)
		}
		slog.Info("Services watched", slog.Int("count", len(services.Items())))
	}
}
