package ntp

import (
	"fmt"
	"testing"
)

func TestCheckLocalTimeAccurate(t *testing.T) {
	accurate, err := CheckLocalTimeAccurate("0.pool.ntp.org")
	if !accurate {
		t.Fatalf("local time is not accurate: %v\n", err)
	}

	accurate, err = CheckLocalTimeAccurate("wrong.ip")
	if accurate {
		t.Fatalf("query ntp pool should failed: %v\n", err)
	}
}

func TestCurrentTime(t *testing.T) {
	tt := CurrentTime("1.pool.ntp.org")
	fmt.Printf("Current Time: %v\n", tt)
	return
}
