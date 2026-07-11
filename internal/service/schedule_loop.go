package service

import (
	"context"
	"time"
)

func runAlignedScheduleLoop(ctx context.Context, tick func(time.Time)) {
	tick(time.Now())
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	nudgeTicker := time.NewTicker(time.Second)
	defer nudgeTicker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			tick(now)
		case <-nudgeTicker.C:
			if ConsumeScheduleNudge() {
				tick(time.Now())
			}
		}
	}
}
