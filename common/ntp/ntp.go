package ntp

import (
	"fmt"
	"os"
	"strings"
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
	errDriftTooMuch          = errors.New("local time drift off ntp server more than 30 seconds")
	errDriftInRange          = errors.New("local time drift off ntp server more than 10 seconds")
	errAllNtpServersTimedOut = errors.New("querying all NTP servers timed out")
	errNtpServersFailed      = errors.New("querying all NTP servers failed")
)

type ClockStatus int

const (
	ClockIsAccurate ClockStatus = iota
	ClockIsNotAccurate
	ClockInWarningRange
	AllNtpServersTimedOut
	NtpServersFailed
)

func (s ClockStatus) Error() error {
	return [...]error{nil, errDriftTooMuch, errDriftInRange, errAllNtpServersTimedOut, errNtpServersFailed}[s]
}

func (s ClockStatus) IsAccurate() bool {
	return s == ClockIsAccurate || s == ClockInWarningRange
}

func (s ClockStatus) IsInWarningRange() bool {
	return s == ClockInWarningRange
}

func (s ClockStatus) AllNtpServersTimedOut() bool {
	return s == AllNtpServersTimedOut
}

func (s ClockStatus) NtpFailed() bool {
	return s == ClockInWarningRange
}

// CheckLocalTimeAccurate returns whether the local clock accurate or not
func CheckLocalTimeAccurate(ntpServers string) (ClockStatus, error) {
	options := beevik_ntp.QueryOptions{Timeout: 30 * time.Second}
	servers := strings.Split(ntpServers, ",")
	// store error messages for each failed NTP server
	// It doesn't print out any error if querying one of the servers is successful
	var errorMessages []string
	allServersTimedOut := true

	for _, ntpServer := range servers {
		response, err := beevik_ntp.QueryWithOptions(strings.TrimSpace(ntpServer), options)
		if err != nil {
			if os.IsTimeout(err) {
				errorMessages = append(errorMessages, fmt.Sprintf("Error querying NTP server %s: timed out", ntpServer))
			} else {
				allServersTimedOut = false
				errorMessages = append(errorMessages, fmt.Sprintf("Error querying NTP server %s: %v", ntpServer, err))
				errorMessages = append(errorMessages, fmt.Sprintf("querying NTP server %v failed. Please config NTP properly", ntpServer))
			}
			continue
		}
		// drift too much
		if response.ClockOffset > toleranceRangeError {
			return ClockIsNotAccurate, nil
		}
		// drift in warning range
		if response.ClockOffset > toleranceRangeWarning {
			return ClockInWarningRange, nil
		}
		return ClockIsAccurate, nil
	}

	// If all servers timed out, it can be a network issue
	// in this case we can continue and it doesn't return error
	if allServersTimedOut {
		return AllNtpServersTimedOut, nil
	}

	// If all servers failed, return a combined error message
	if len(errorMessages) > 0 {
		return NtpServersFailed, fmt.Errorf("querying NTP servers:\n%s", strings.Join(errorMessages, "\n"))
	}

	return NtpServersFailed, fmt.Errorf("unknown failure")
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
