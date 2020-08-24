package herrors

import (
	"errors"
	"testing"
)

var (
	err1 = errors.New("err1")
	err2 = errors.New("err2")
	err3 = errors.New("err3")
)

func TestJoin(t *testing.T) {
	tests := []struct {
		errs   []error
		expStr string
	}{
		{
			errs:   nil,
			expStr: "",
		},
		{
			errs:   []error{err1},
			expStr: "err1",
		},
		{
			errs:   []error{err1, nil, err2},
			expStr: "err1; err2",
		},
		{
			errs:   []error{err1, err2, err3},
			expStr: "err1; err2; err3",
		},
	}

	for i, test := range tests {
		got := Join(test.errs...)
		if (got == nil) != (len(test.expStr) == 0) {
			t.Fatalf("unexpected nil value")
		}
		if got != nil && got.Error() != test.expStr {
			t.Errorf("Test %v: unexpected error string [%v] / [%v]", i, got.Error(), test.expStr)
		}
	}
}
