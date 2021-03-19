package requestmanager

import (
	"container/list"
	"fmt"
	"strings"
	"testing"

	"github.com/pkg/errors"
)

func TestRequestQueue_Push(t *testing.T) {
	tests := []struct {
		initSize []int
		priority reqPriority
		expSize  []int
		expErr   error
	}{
		{
			initSize: []int{10, 10},
			priority: reqPriorityHigh,
			expSize:  []int{11, 10},
			expErr:   nil,
		},
		{
			initSize: []int{10, 10},
			priority: reqPriorityLow,
			expSize:  []int{10, 11},
			expErr:   nil,
		},
		{
			initSize: []int{maxWaitingSize, maxWaitingSize},
			priority: reqPriorityLow,
			expErr:   ErrQueueFull,
		},
		{
			initSize: []int{maxWaitingSize, maxWaitingSize},
			priority: reqPriorityHigh,
			expErr:   ErrQueueFull,
		},
	}
	for i, test := range tests {
		q := makeTestRequestQueue(test.initSize)
		req := wrapRequestFromRaw(makeTestRequest(100))

		err := q.Push(req, test.priority)
		if assErr := assertError(err, test.expErr); assErr != nil {
			t.Errorf("Test %v: %v", i, assErr)
		}
		if err != nil || test.expErr != nil {
			continue
		}

		if err := q.checkSizes(test.expSize); err != nil {
			t.Errorf("Test %v: %v", i, err)
		}
	}
}

func TestRequestQueue_Pop(t *testing.T) {
	tests := []struct {
		initSizes []int
		expNil    bool
		expIndex  uint64
		expSizes  []int
	}{
		{
			initSizes: []int{10, 10},
			expNil:    false,
			expIndex:  0,
			expSizes:  []int{9, 10},
		},
		{
			initSizes: []int{0, 10},
			expNil:    false,
			expIndex:  0,
			expSizes:  []int{0, 9},
		},
		{
			initSizes: []int{0, 0},
			expNil:    true,
			expSizes:  []int{0, 0},
		},
	}
	for i, test := range tests {
		q := makeTestRequestQueue(test.initSizes)
		req := q.Pop()

		if err := q.checkSizes(test.expSizes); err != nil {
			t.Errorf("Test %v: %v", i, err)
		}
		if req == nil != (test.expNil) {
			t.Errorf("test %v: unpected nil", i)
		}
		if req == nil {
			continue
		}
		index := req.Request.(*testRequest).index
		if index != test.expIndex {
			t.Errorf("Test %v: unexpected index: %v / %v", i, index, test.expIndex)
		}
	}
}

func makeTestRequestQueue(sizes []int) requestQueues {
	if len(sizes) != 2 {
		panic("unexpected sizes")
	}
	q := newRequestQueues()

	index := 0
	for i := 0; i != sizes[0]; i++ {
		q.reqsPHigh.push(wrapRequestFromRaw(makeTestRequest(uint64(index))))
		index++
	}
	for i := 0; i != sizes[1]; i++ {
		q.reqsPLow.push(wrapRequestFromRaw(makeTestRequest(uint64(index))))
		index++
	}
	return q
}

func wrapRequestFromRaw(raw *testRequest) *request {
	return &request{
		Request: raw,
	}
}

func getTestRequestFromElem(elem *list.Element) (*testRequest, error) {
	req, ok := elem.Value.(*request)
	if !ok {
		return nil, errors.New("unexpected type")
	}
	raw, ok := req.Request.(*testRequest)
	if !ok {
		return nil, errors.New("unexpected raw types")
	}
	return raw, nil
}

func (q *requestQueues) checkSizes(sizes []int) error {
	if len(sizes) != 2 {
		panic("expect 2 sizes")
	}
	if q.reqsPHigh.len() != sizes[0] {
		return fmt.Errorf("high priority %v / %v", q.reqsPHigh.len(), sizes[0])
	}
	if q.reqsPLow.len() != sizes[1] {
		return fmt.Errorf("low priority %v / %v", q.reqsPLow.len(), sizes[2])
	}
	return nil
}

func assertError(got, exp error) error {
	if (got == nil) != (exp == nil) {
		return fmt.Errorf("unexpected error: %v / %v", got, exp)
	}
	if got == nil {
		return nil
	}
	if !strings.Contains(got.Error(), exp.Error()) {
		return fmt.Errorf("unexpected error: %v / %v", got, exp)
	}
	return nil
}
