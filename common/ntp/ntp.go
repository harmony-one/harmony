package ntp

import (
	"time"

	beevik_ntp "github.com/beevik/ntp"
	"github.com/pkg/errors"
)

const (
	// tolerance range of local clock with NTP time server
	toleranceRangeWarning = time.Duration(10 * time.Second)
	toleranceRangeError   = time.Duration(30 * time.Second)
)

var (
	errDriftTooMuch = errors.New("local time drift off ntp server more than 30 seconds")
	errDriftInRange = errors.New("local time drift off ntp server more than 10 seconds")
)

// CheckLocalTimeAccurate returns whether the local clock accurate or not
func CheckLocalTimeAccurate(ntpServer string) (bool, error) {
	options := beevik_ntp.QueryOptions{Timeout: 10 * time.Second}
	response, err := beevik_ntp.QueryWithOptions(ntpServer, options)
	// failed to query ntp time
	if err != nil {
		return false, err
	}
	// drift too much
	if response.ClockOffset > toleranceRangeError {
		return false, errDriftTooMuch
	}
	if response.ClockOffset > toleranceRangeWarning {
		return true, errDriftInRange
	}
	return true, nil
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
