package ntptime

import "time"

// LocalTime implements NTPTime using the local system clock without any correction.
// Used as a fallback when NTP is unavailable (e.g. localnet).
type LocalTime struct {
}

func (LocalTime) Now() time.Time {
	return time.Now()
}
