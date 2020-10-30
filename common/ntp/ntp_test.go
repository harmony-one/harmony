package ntp

import (
	"fmt"
	"os"
	"testing"
)

func TestCheckLocalTimeAccurate(t *testing.T) {
	accurate, err := CheckLocalTimeAccurate("wrong.ip")
	if accurate {
		t.Fatalf("query ntp pool should failed: %v\n", err)
	}

	accurate, err = CheckLocalTimeAccurate("0.pool.ntp.org")
	if !accurate {
		if os.IsTimeout(err) {
			t.Skip(err)
		}
		t.Fatal(err)
	}
}

func TestCurrentTime(t *testing.T) {
	tt := CurrentTime("1.pool.ntp.org")
	fmt.Printf("Current Time: %v\n", tt)
	return
}
