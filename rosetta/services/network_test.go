package services

import (
	"fmt"
	"reflect"
	"sort"
	"testing"

	"github.com/coinbase/rosetta-sdk-go/types"
	"github.com/harmony-one/harmony/rosetta/common"
	commonRPC "github.com/harmony-one/harmony/rpc/common"
	"github.com/libp2p/go-libp2p/core/peer"
)

func TestErrors(t *testing.T) {
	refBaseErrors := []*types.Error{
		&common.CatchAllError,
		&common.SanityCheckError,
		&common.InvalidNetworkError,
		&common.TransactionSubmissionError,
		&common.BlockNotFoundError,
		&common.TransactionNotFoundError,
		&common.ReceiptNotFoundError,
		&common.UnsupportedCurveTypeError,
		&common.InvalidTransactionConstructionError,
	}
	refBeaconErrors := []*types.Error{
		&common.StakingTransactionSubmissionError,
	}

	if !reflect.DeepEqual(refBaseErrors, getErrors()) {
		t.Errorf("Expected errors to be: %v", refBaseErrors)
	}
	if !reflect.DeepEqual(refBeaconErrors, getBeaconErrors()) {
		t.Errorf("Expected errors to be: %v", refBeaconErrors)
	}
}

func TestOperationStatus(t *testing.T) {
	refBaseOperations := []*types.OperationStatus{
		common.SuccessOperationStatus,
		common.FailureOperationStatus,
		common.ContractFailureOperationStatus,
	}
	refBeaconOperations := []*types.OperationStatus{}

	if !reflect.DeepEqual(refBaseOperations, getOperationStatuses()) {
		t.Errorf("Expected operation status to be: %v", refBaseOperations)
	}
	if !reflect.DeepEqual(refBeaconOperations, getBeaconOperationStatuses()) {
		t.Errorf("Expected operation status to be: %v", refBeaconOperations)
	}
}

// Note that this assumes TestErrors & TestOperationStatus passes
func TestAllow(t *testing.T) {
	refBaseAllow := &types.Allow{
		OperationStatuses:       getOperationStatuses(),
		OperationTypes:          common.PlainOperationTypes,
		Errors:                  getErrors(),
		HistoricalBalanceLookup: false,
	}
	refBeaconAllow := &types.Allow{
		OperationStatuses:       append(getOperationStatuses(), getBeaconOperationStatuses()...),
		OperationTypes:          append(common.PlainOperationTypes, common.StakingOperationTypes...),
		Errors:                  append(getErrors(), getBeaconErrors()...),
		HistoricalBalanceLookup: false,
	}

	if !reflect.DeepEqual(refBaseAllow, getAllow(false)) {
		t.Errorf("Expected allow to be: %v", refBaseAllow)
	}
	if !reflect.DeepEqual(refBeaconAllow, getBeaconAllow(false)) {
		t.Errorf("Expected allow to be: %v", refBeaconAllow)
	}
}

func TestValidAssertValidNetworkIdentifier(t *testing.T) {
	testNetworkID, err := common.GetNetwork(0)
	if err != nil {
		t.Fatal(err.Error())
	}

	if err := assertValidNetworkIdentifier(testNetworkID, 0); err != nil {
		t.Error(err)
	}
}

func TestInvalidAssertValidNetworkIdentifier(t *testing.T) {
	testNetworkID, err := common.GetNetwork(0)
	if err != nil {
		t.Fatal(err.Error())
	}
	if err := assertValidNetworkIdentifier(testNetworkID, 0); err != nil {
		t.Fatal(err)
	}

	testNetworkID.Blockchain = "Bad"
	rosettaError := assertValidNetworkIdentifier(testNetworkID, 0)
	if rosettaError == nil {
		t.Error("Expected error for bad blockchain for network ID")
	} else if rosettaError.Code != common.InvalidNetworkError.Code {
		t.Errorf("Expected returned error code to be %v", common.InvalidNetworkError.Code)
	}

	testNetworkID, _ = common.GetNetwork(0)
	testNetworkID.Network = "not a network"
	rosettaError = assertValidNetworkIdentifier(testNetworkID, 0)
	if rosettaError == nil {
		t.Error("Expected error for bad network for network ID")
	} else if rosettaError.Code != common.InvalidNetworkError.Code {
		t.Errorf("Expected returned error code to be %v", common.InvalidNetworkError.Code)
	}

	testNetworkID, _ = common.GetNetwork(0)
	testNetworkID.SubNetworkIdentifier.Network = "not a shard"
	rosettaError = assertValidNetworkIdentifier(testNetworkID, 0)
	if rosettaError == nil {
		t.Error("Expected error for bad subnetwork for network ID")
	} else if rosettaError.Code != common.InvalidNetworkError.Code {
		t.Errorf("Expected returned error code to be %v", common.InvalidNetworkError.Code)
	}

	testNetworkID, _ = common.GetNetwork(0)
	testNetworkID.SubNetworkIdentifier.Metadata = map[string]interface{}{"blah": "blah"}
	rosettaError = assertValidNetworkIdentifier(testNetworkID, 0)
	if rosettaError == nil {
		t.Error("Expected error for bad subnetwork metadata for network ID")
	} else if rosettaError.Code != common.InvalidNetworkError.Code {
		t.Errorf("Expected returned error code to be %v", common.InvalidNetworkError.Code)
	}

	testNetworkID, _ = common.GetNetwork(0)
	isBeacon, ok := testNetworkID.SubNetworkIdentifier.Metadata["is_beacon"].(bool)
	if !ok {
		t.Fatal("Could not get `is_beacon` subnetwork metadata from reference network ID")
	}
	testNetworkID.SubNetworkIdentifier.Metadata["is_beacon"] = !isBeacon
	rosettaError = assertValidNetworkIdentifier(testNetworkID, 0)
	if rosettaError == nil {
		t.Error("Expected error for bad subnetwork metadata for network ID")
	} else if rosettaError.Code != common.InvalidNetworkError.Code {
		t.Errorf("Expected returned error code to be %v", common.InvalidNetworkError.Code)
	}
}

func TestGetPeersFromNodePeerInfo(t *testing.T) {
	testNodePeerInfo := generateTestNodePeerInfoList()
	peers, err := getPeersFromNodePeerInfo(testNodePeerInfo)
	if err != nil {
		t.Fatal(err)
	}

	found := map[string]bool{}
	for _, p := range peers {
		found[p.PeerID] = true

		var refTopics []string
		switch p.PeerID {
		case peer.ID("jim").String():
			refTopics = []string{"test1", "test2"}
		case peer.ID("bob").String():
			refTopics = []string{"test1"}
		case peer.ID("alice").String():
			refTopics = []string{"test3"}
		case peer.ID("mary").String():
			refTopics = []string{"test3"}
		default:
			t.Fatalf("unknown peerID %v", p.PeerID)
		}

		if err := checkPeerID(p, refTopics); err != nil {
			t.Errorf(err.Error())
		}
	}

	if len(found) != 4 {
		t.Errorf("Did not find peerIDs for all 4 peers: %v", found)
	}
}

func generateTestNodePeerInfoList() commonRPC.NodePeerInfo {
	return commonRPC.NodePeerInfo{
		PeerID:       "test",
		BlockedPeers: []peer.ID{},
		P: []commonRPC.P{
			{
				Topic: "test1",
				Peers: []peer.ID{
					"jim",
					"bob",
				},
			},
			{
				Topic: "test2",
				Peers: []peer.ID{
					"jim",
				},
			},
			{
				Topic: "test3",
				Peers: []peer.ID{
					"alice",
					"mary",
				},
			},
		},
	}
}

func checkPeerID(p *types.Peer, refTopics []string) error {
	topics, ok := p.Metadata["topics"].([]string)
	if !ok {
		return fmt.Errorf("expected topics in metadata of %v to be a slice of string", p)
	}
	sort.Strings(topics)
	sort.Strings(refTopics)
	if !reflect.DeepEqual(topics, refTopics) {
		return fmt.Errorf("topics %v does not match reference topics %v", topics, refTopics)
	}
	return nil
}
