package downloader_test

import (
	"reflect"
	"testing"

	bc "github.com/harmony-one/harmony/blockchain"
	"github.com/harmony-one/harmony/crypto/pki"
	"github.com/harmony-one/harmony/services/syncing/downloader"
	pb "github.com/harmony-one/harmony/services/syncing/downloader/proto"
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

// GetBlocks used for state download.
func (node *FakeNode) GetBlocks() [][]byte {
	res := [][]byte{}
	for _, block := range node.bc.Blocks {
		res = append(res, block.Serialize())
	}
	return res
}

// SetBlockchain is used for testing
func (node *FakeNode) Init() {
	addresses := [][20]byte{TestAddressOne, TestAddressTwo}
	node.bc = bc.CreateBlockchainWithMoreBlocks(addresses, ShardID)
}

// CalculateResponse is the implementation for DownloadInterface.
func (node *FakeNode) CalculateResponse(request *pb.DownloaderRequest) (*pb.DownloaderResponse, error) {
	response := &pb.DownloaderResponse{}
	if request.Type == pb.DownloaderRequest_HEADER {
		for _, block := range node.bc.Blocks {
			response.Payload = append(response.Payload, block.Hash[:])
		}
	} else {
		for i := range request.Hashes {
			block := node.bc.FindBlock(request.Hashes[i])
			response.Payload = append(response.Payload, block.Serialize())
		}
	}
	return response, nil
}

// TestGetBlockHashes tests GetBlockHashes function.
func TestGetBlockHashes(t *testing.T) {
	fakeNode := &FakeNode{}
	fakeNode.Init()
	s := downloader.NewServer(fakeNode)
	grcpServer, err := s.Start(serverIP, serverPort)
	if err != nil {
		t.Error(err)
	}
	defer grcpServer.Stop()

	client := downloader.ClientSetup(serverIP, serverPort)
	defer client.Close()
	response := client.GetBlockHashes()
	if !reflect.DeepEqual(response.Payload, fakeNode.GetBlockHashes()) {
		t.Error("not equal")
	}
}

// TestGetBlocks tests GetBlocks function.
func TestGetBlocks(t *testing.T) {
	fakeNode := &FakeNode{}
	fakeNode.Init()
	s := downloader.NewServer(fakeNode)
	grcpServer, err := s.Start(serverIP, serverPort)
	if err != nil {
		t.Error(err)
	}
	defer grcpServer.Stop()

	client := downloader.ClientSetup(serverIP, serverPort)
	defer client.Close()
	response := client.GetBlockHashes()
	if !reflect.DeepEqual(response.Payload, fakeNode.GetBlockHashes()) {
		t.Error("not equal")
	}
	response = client.GetBlocks([][]byte{response.Payload[0], response.Payload[1]})
	if !reflect.DeepEqual(response.Payload, fakeNode.GetBlocks()) {
		t.Error("not equal")
	}
}
