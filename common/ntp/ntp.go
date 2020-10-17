package ntp

import (
	"time"

	beevik_ntp "github.com/beevik/ntp"
)

const (
	// tolerance range of local clock with NTP time server
	toleranceRange = time.Duration(50 * time.Millisecond)
)

// IsLocalTimeAccurate returns whether the local clock acturate or not
func IsLocalTimeAccurate(ntpServer string) bool {
	response, err := beevik_ntp.Query(ntpServer)
	// failed to query ntp time, return false
	if err != nil {
		return false
	}
	if response.ClockOffset > toleranceRange {
		return false
	}
	return true
}

// CurrentTime return the current time calibrated using ntp server
func CurrentTime(ntpServer string) time.Time {
	options := beevik_ntp.QueryOptions{Timeout: 2 * time.Second}
	t, err := beevik_ntp.QueryWithOptions(ntpServer, options)
	if err == nil {
		return t.Time
	}
	return time.Now()
}
