package ntptime

import "time"

type LocalTime struct {
}

func (LocalTime) Now() time.Time {
	return time.Now()
}
