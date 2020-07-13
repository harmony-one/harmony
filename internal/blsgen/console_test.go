package blsgen

import (
	"errors"
	"fmt"
	"time"
)

func setTestConsole(tc *testConsole) {
	console = tc
}

type testConsole struct {
	In  chan string
	Out chan string
}

func newTestConsole() *testConsole {
	in := make(chan string, 100)
	out := make(chan string, 100)
	return &testConsole{in, out}
}

func (tc *testConsole) readPassword() (string, error) {
	return tc.readln()
}

func (tc *testConsole) readln() (string, error) {
	select {
	case <-time.After(2 * time.Second):
		return "", errors.New("timed out")
	case msg, ok := <-tc.In:
		if !ok {
			return "", errors.New("in channel closed")
		}
		return msg, nil
	}
}

func (tc *testConsole) print(a ...interface{}) {
	msg := fmt.Sprint(a...)
	tc.Out <- msg
}

func (tc *testConsole) println(a ...interface{}) {
	msg := fmt.Sprintln(a...)
	tc.Out <- msg
}

func (tc *testConsole) printf(format string, a ...interface{}) {
	msg := fmt.Sprintf(format, a...)
	tc.Out <- msg
}

func (tc *testConsole) checkClean() (bool, string) {
	select {
	case msg := <-tc.In:
		return false, "extra in message: " + msg
	case msg := <-tc.Out:
		return false, "extra out message: " + msg
	default:
		return true, ""
	}
}

func (tc *testConsole) checkOutput(timeout time.Duration, checkFunc func(string) error) error {
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	select {
	case <-time.After(timeout):
		return errors.New("timed out")
	case msg := <-tc.Out:
		return checkFunc(msg)
	}
}
