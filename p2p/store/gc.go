package store

import (
	"context"
	"sync"
	"time"

	"github.com/harmony-one/harmony/common/clock"
	"github.com/harmony-one/harmony/internal/utils"
)

const (
	gcPeriod = 2 * time.Hour
)

type gcAction func() error

func startGc(ctx context.Context, clock clock.Clock, bgTasks *sync.WaitGroup, action gcAction) {
	bgTasks.Add(1)
	go func() {
		defer bgTasks.Done()

		gcTimer := clock.NewTicker(gcPeriod)
		defer gcTimer.Stop()

		for {
			select {
			case <-gcTimer.Ch():
				if err := action(); err != nil {
					utils.Logger().Warn().Err(err).Msg("GC failed")
				}

			case <-ctx.Done():
				return
			}
		}
	}()
}
