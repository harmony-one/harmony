package ntptime

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/beevik/ntp"
	"github.com/harmony-one/harmony/internal/utils"
)

// NTPTime returns the current time, optionally corrected by an NTP clock offset.
type NTPTime interface {
	Now() time.Time
}

// inner abstracts the NTP query so it can be replaced in tests.
type inner interface {
	Query(addr string) (*ntp.Response, error)
}

type ntpInner struct {
}

func (ntpInner) Query(addr string) (*ntp.Response, error) {
	rsp, err := ntp.QueryWithOptions(addr, ntp.QueryOptions{})
	if err != nil {
		return nil, err
	}
	err = rsp.Validate()
	if err != nil {
		return nil, err
	}
	return rsp, nil
}

// ntpTimeImpl holds the latest clock offset measured against an NTP server
// and applies it to every Now() call.
type ntpTimeImpl struct {
	mu     sync.RWMutex
	offset time.Duration // difference between NTP time and local clock
	addr   string        // active server chosen at startup
	inner  inner
}

// TryNew creates an ntpTimeImpl from a comma-separated list of NTP server
// addresses, retrying up to tries times on failure.
func TryNew(ntpServers string, tries uint) (*ntpTimeImpl, error) {
	return tryNew(ntpServers, tries, ntpInner{})
}

func tryNew(ntpServers string, tries uint, inner inner) (*ntpTimeImpl, error) {
	if tries == 0 {
		return newNTPTime(ntpServers, inner)
	}
	rs, err := newNTPTime(ntpServers, inner)
	if err != nil {
		return tryNew(ntpServers, tries-1, inner)
	}
	return rs, nil
}

func newNTPTime(ntpServers string, inner inner) (*ntpTimeImpl, error) {
	addrs := parseServers(ntpServers)
	if len(addrs) == 0 {
		return nil, fmt.Errorf("no NTP servers provided")
	}
	// Shuffle so every node doesn't hammer the same server.
	rand.Shuffle(len(addrs), func(i, j int) { addrs[i], addrs[j] = addrs[j], addrs[i] })
	for _, addr := range addrs {
		tm, err := inner.Query(addr)
		if err != nil {
			continue
		}
		return &ntpTimeImpl{
			addr:   addr,
			inner:  inner,
			offset: tm.ClockOffset,
		}, nil
	}
	return nil, fmt.Errorf("all NTP servers unreachable")
}

// parseServers splits a comma-separated server list and trims whitespace.
func parseServers(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// Run periodically refreshes the NTP clock offset every duration.
// It should be started in a goroutine and will run until ctx is cancelled.
func (a *ntpTimeImpl) Run(ctx context.Context, duration time.Duration) {
	for {
		timer := time.NewTimer(duration)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return
		case <-timer.C:
			tm, err := a.inner.Query(a.addr)
			if err != nil {
				utils.Logger().Info().Msgf("Failed to run NTP time provider: %v", err)
				continue
			}
			a.mu.Lock()
			a.offset = tm.ClockOffset
			a.mu.Unlock()
		}
	}
}

// Now returns the current local time corrected by the latest NTP offset.
func (a *ntpTimeImpl) Now() time.Time {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return time.Now().Add(a.offset)
}
