package ntp

import (
	"fmt"
	"testing"
)

func TestCheckLocalTimeAccurate(t *testing.T) {
	response, err := CheckLocalTimeAccurate("wrong.ip")
	if response.IsAccurate() {
		if err != nil {
			t.Fatalf("query ntp pool should failed: %v\n", err)
		}
		t.Fatalf("query ntp pool should failed: %s\n", response.Message())
	}
	if err == nil {
		t.Fatalf("query invalid ntp pool should return error\n")
	}

	response, err = CheckLocalTimeAccurate("0.pool.ntp.org")
	if !response.IsAccurate() || err != nil {
		if response.AllNtpServersTimedOut() {
			t.Skip(response.Message())
		}
		if err != nil {
			t.Fatalf("query valid ntp pool shouldn't return error: %v\n", err)
		}
		t.Fatalf("local clock is not accurate: %v\n", response.Message())
	}

	// test multiple ntp servers, the second one is a valid server
	response, err = CheckLocalTimeAccurate("wrong.ip,0.pool.ntp.org")
	if !response.IsAccurate() || err != nil {
		if response.AllNtpServersTimedOut() {
			t.Skip(response.Message())
		}
		if err != nil {
			t.Fatalf("query multiple ntp pools shouldn't return error: %v\n", err)
		}
		t.Fatalf("local clock is not accurate: %v\n", response.Message())
	}

	// test multiple ntp servers, both invalid server
	response, err = CheckLocalTimeAccurate("1.wrong.ip,2.wrong.ip")
	if response.IsAccurate() || err == nil {
		t.Fatal("invalid servers can't respond or validate time")
	}
}

func TestCurrentTime(t *testing.T) {
	tt := CurrentTime("1.pool.ntp.org")
	fmt.Printf("Current Time: %v\n", tt)
}
