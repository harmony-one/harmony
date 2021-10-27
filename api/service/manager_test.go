package service

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/harmony-one/harmony/eth/rpc"
)

func TestManager_StartServices(t *testing.T) {
	tests := []struct {
		services []Service
		stopped  bool
		err      error
	}{
		{
			services: []Service{
				makeTestService(0, nil, nil),
				makeTestService(1, nil, nil),
				makeTestService(2, nil, nil),
			},
			stopped: false,
			err:     nil,
		},
		{
			services: []Service{
				makeTestService(0, func() error { return errors.New("start error") }, nil),
			},
			stopped: true,
			err:     errors.New("cannot start service [Unknown]: start error"),
		},
		{
			services: []Service{
				makeTestService(0, nil, nil),
				makeTestService(1, nil, nil),
				makeTestService(2, func() error { return errors.New("start error") }, nil),
			},
			stopped: true,
			err:     errors.New("cannot start service [Unknown]: start error"),
		},
		{
			services: []Service{
				makeTestService(0, nil, nil),
				makeTestService(1, nil, func() error { return errors.New("stop error") }),
				makeTestService(2, func() error { return errors.New("start error") }, nil),
			},
			stopped: true,
			err:     errors.New("cannot start service [Unknown]: start error; failed to stop service [Unknown]: stop error"),
		},
	}
	for i, test := range tests {
		m := &Manager{
			services: test.services,
		}
		err := m.StartServices()
		if assErr := assertError(err, test.err); assErr != nil {
			t.Errorf("Test %v: unexpected error: %v", i, assErr)
		}
		for _, s := range test.services {
			ts := s.(*testService)
			if ts.started == test.stopped {
				t.Errorf("Test %v: [service %v] test status unexpected", i, ts.index)
			}
		}
	}
}

func TestManager_StopServices(t *testing.T) {
	tests := []struct {
		services []Service
		expErr   error
	}{
		{
			services: []Service{
				makeTestService(0, nil, nil),
				makeTestService(1, nil, nil),
				makeTestService(2, nil, nil),
			},
			expErr: nil,
		},
		{
			services: []Service{
				makeTestService(0, nil, nil),
				makeTestService(1, nil, func() error { return errors.New("expect error") }),
				makeTestService(2, nil, func() error { return errors.New("expect error") }),
			},
			expErr: errors.New("failed to stop service [Unknown]: expect error; failed to stop service [Unknown]: expect error"),
		},
	}

	for i, test := range tests {
		m := &Manager{
			services: test.services,
		}
		err := m.StopServices()
		if assErr := assertError(err, test.expErr); assErr != nil {
			t.Errorf("Test %v: %v", i, assErr)
		}
		for _, s := range test.services {
			ts := s.(*testService)
			if ts.started {
				t.Errorf("Test %v: Service%v not stopped", i, ts.index)
			}
		}
	}

}

type testService struct {
	index        int
	started      bool
	startErrHook func() error
	stopErrHook  func() error
}

func makeTestService(index int, startErrHook, stopErrHook func() error) *testService {
	return &testService{
		index:        index,
		startErrHook: startErrHook,
		stopErrHook:  stopErrHook,
	}
}

func (s *testService) Start() error {
	if s.startErrHook != nil {
		if err := s.startErrHook(); err != nil {
			return err
		}
	}
	s.started = true
	return nil
}

func (s *testService) Stop() error {
	if s.stopErrHook != nil {
		if err := s.stopErrHook(); err != nil {
			s.started = false
			return err
		}
	}
	s.started = false
	return nil
}

func (s *testService) APIs() []rpc.API {
	return nil
}

func assertError(got, expect error) error {
	if (got == nil) != (expect == nil) {
		return fmt.Errorf("unexpected error [%v] / [%v]", got, expect)
	}
	if (got == nil) || (expect == nil) {
		return nil
	}
	if !strings.Contains(got.Error(), expect.Error()) {
		return fmt.Errorf("unexpected error [%v] / [%v]", got, expect)
	}
	return nil
}
