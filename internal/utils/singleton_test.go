package utils

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFileNo(t *testing.T) {
	f1 := FileNo()
	f2 := FileNo()
	require.NotEqual(t, f1, f2)
}
