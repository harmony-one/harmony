package downloader

import (
	"fmt"
	"reflect"
	"testing"

	bc "github.com/harmony-one/harmony/blockchain"
	"github.com/harmony-one/harmony/crypto/pki"
	pb "github.com/harmony-one/harmony/syncing/downloader/proto"
)

const (
	serverPort = "9997"
	serverIP   = "127.0.0.1"
	clientPort = "9999"
)

var (
	PriIntOne      = 111
	PriIntTwo      = 222
	TestAddressOne = pki.GetAddressFromInt(PriIntOne)
	TestAddressTwo = pki.GetAddressFromInt(PriIntTwo)
	ShardID        = uint32(0)
)

type FakeNode struct {
	bc *bc.Blockchain
}

// GetBlockHashes used for state download.
func (node *FakeNode) GetBlockHashes() [][]byte {
	res := [][]byte{}
	for _, block := range node.bc.Blocks {
		res = append(res, block.Hash[:])
	}
	return res
}

// SetBlockchain is used for testing
func (node *FakeNode) Init() {
	addresses := [][20]byte{TestAddressOne, TestAddressTwo}
	node.bc = bc.CreateBlockchainWithMoreBlocks(addresses, ShardID)
}

func (node *FakeNode) CalculateResponse(request *pb.DownloaderRequest) (*pb.DownloaderResponse, error) {
	response := &pb.DownloaderResponse{}
	if request.Type == pb.DownloaderRequest_HEADER {
		fmt.Println("minh ", len(node.bc.Blocks))
		for _, block := range node.bc.Blocks {
			response.Payload = append(response.Payload, block.Hash[:])
		}
	} else {

	}
	return response, nil
}

func TestGetBlockHashes(t *testing.T) {
	fakeNode := &FakeNode{}
	fakeNode.Init()
	s := NewServer(fakeNode)
	grcpServer, err := s.Start(serverIP, serverPort)
	if err != nil {
		t.Error(err)
	}
	defer grcpServer.Stop()

	client := ClientSetup(serverIP, serverPort)
	response := client.GetBlockHashes()
	if !reflect.DeepEqual(response.Payload, fakeNode.GetBlockHashes()) {
		t.Error("not equal")
	}

	defer client.Close()
}
