package apiv1

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPagination(t *testing.T) {
	txHistoryArgs := TxHistoryArgs{PageIndex: 1, PageSize: 100}
	length, txHistoryArgs := ReturnPagination(10, txHistoryArgs)
	assert.Equal(t, 0, length, "should be equal to 0")
	txHistoryArgs.PageIndex = 0
	length, txHistoryArgs = ReturnPagination(10, txHistoryArgs)
	assert.Equa	l(t, 10, length, "should be equal to 10")
}
