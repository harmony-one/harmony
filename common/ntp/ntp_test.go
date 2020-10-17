package ntp

import (
	"fmt"
	"testing"
)

func TestIsLocalTimeAccurate(t *testing.T) {
	if !IsLocalTimeAccurate("0.pool.ntp.org") {
		t.Fatal("local time is not accurate")
	}

	if IsLocalTimeAccurate("wrong.ip") {
		t.Fatal("query ntp pool should failed")
	}
}

func TestCurrentTime(t *testing.T) {
	tt := CurrentTime("1.pool.ntp.org")
	fmt.Printf("Current Time: %v\n", tt)
	return
}
