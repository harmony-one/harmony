package genesis

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestKeyExists(t *testing.T) {
	require.NotEmpty(t, ContractDeployerKey)
}
