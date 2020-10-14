package types

import (
	"reflect"
	"testing"

	ethcommon "github.com/ethereum/go-ethereum/common"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/harmony-one/harmony/staking"
)

func TestFindLogsWithTopic(t *testing.T) {
	tests := []struct {
		receipt          *Receipt
		topic            ethcommon.Hash
		expectedResponse []*Log
	}{
		// test 0
		{
			receipt: &Receipt{
				Logs: []*Log{
					{
						Topics: []ethcommon.Hash{
							staking.IsValidatorKey,
							staking.IsValidator,
						},
					},
					{
						Topics: []ethcommon.Hash{
							crypto.Keccak256Hash([]byte("test")),
						},
					},
					{
						Topics: []ethcommon.Hash{
							staking.CollectRewardsTopic,
						},
					},
				},
			},
			topic: staking.IsValidatorKey,
			expectedResponse: []*Log{
				{
					Topics: []ethcommon.Hash{
						staking.IsValidatorKey,
						staking.IsValidator,
					},
				},
			},
		},
		// test 1
		{
			receipt: &Receipt{
				Logs: []*Log{
					{
						Topics: []ethcommon.Hash{
							staking.IsValidatorKey,
							staking.IsValidator,
						},
					},
					{
						Topics: []ethcommon.Hash{
							crypto.Keccak256Hash([]byte("test")),
						},
					},
					{
						Topics: []ethcommon.Hash{
							staking.CollectRewardsTopic,
						},
					},
				},
			},
			topic: staking.CollectRewardsTopic,
			expectedResponse: []*Log{
				{
					Topics: []ethcommon.Hash{
						staking.CollectRewardsTopic,
					},
				},
			},
		},
		// test 2
		{
			receipt: &Receipt{
				Logs: []*Log{
					{
						Topics: []ethcommon.Hash{
							staking.IsValidatorKey,
						},
					},
					{
						Topics: []ethcommon.Hash{
							crypto.Keccak256Hash([]byte("test")),
						},
					},
					{
						Topics: []ethcommon.Hash{
							staking.CollectRewardsTopic,
						},
					},
				},
			},
			topic:            staking.IsValidator,
			expectedResponse: []*Log{},
		},
	}

	for i, test := range tests {
		response := FindLogsWithTopic(test.receipt, test.topic)
		if !reflect.DeepEqual(test.expectedResponse, response) {
			t.Errorf("Failed test %v, expected %v, got %v", i, test.expectedResponse, response)
		}
	}
}
