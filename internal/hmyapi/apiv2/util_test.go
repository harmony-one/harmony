package apiv2

import (
	"testing"
)

func TestPagination(t *testing.T) {
	txHistoryArgs := TxHistoryArgs{PageIndex: 1, PageSize: 100}
	length, txHistoryArgs := ReturnPagination(10, txHistoryArgs)
	if length != 0 {
		t.Errorf("expected pagination length %d; got %d", 0, length)
	}
	txHistoryArgs.PageIndex = 0
	length, txHistoryArgs = ReturnPagination(10, txHistoryArgs)
	if length != 10 {
		t.Errorf("expected pagination length %d; got %d", 10, length)
	}
}
