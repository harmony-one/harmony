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
		t.Fatalf("query ntp pool should failed: %v\n", response.Error())
	}
	if err == nil {
		t.Fatalf("query invalid ntp pool should return error\n")
	}

	response, err = CheckLocalTimeAccurate("0.pool.ntp.org")
	if !response.IsAccurate() || err != nil {
		if response.AllNtpServersTimedOut() {
			t.Skip(response.Error())
		}
		if err != nil {
			t.Fatal(err)
		}
		t.Fatal(response.Error())
	}

	// test multiple ntp servers, the second one is a valid server
	response, err = CheckLocalTimeAccurate("wrong.ip,0.pool.ntp.org")
	if !response.IsAccurate() || err != nil {
		if response.AllNtpServersTimedOut() {
			t.Skip(response.Error())
		}
		if err != nil {
			t.Fatal(err)
		}
		t.Fatal(response.Error())
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
