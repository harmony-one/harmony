package peersexchange_test

import (
	"time"

	"github.com/harmony-one/harmony/api/service/legacysync/downloader"
	pb "github.com/harmony-one/harmony/api/service/legacysync/downloader/proto"
	"google.golang.org/grpc/connectivity"
)

var _ downloader.Client = &client{}

type client struct {
	isReady  bool
	response *pb.DownloaderResponse
	err      error
}

func (c client) IsConnecting() bool {
	//TODO implement me
	panic("implement me")
}

func (c client) State() connectivity.State {
	//TODO implement me
	panic("implement me")
}

func (c client) GetBlockHashes(startHash []byte, size uint32, ip, port string) *pb.DownloaderResponse {
	//TODO implement me
	panic("implement me")
}

func (c client) GetBlockHeaders(hashes [][]byte) *pb.DownloaderResponse {
	//TODO implement me
	panic("implement me")
}

func (c client) Register(hash []byte, ip, port string) *pb.DownloaderResponse {
	//TODO implement me
	panic("implement me")
}

func (c client) GetBlockChainHeight() (*pb.DownloaderResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (c client) GetBlocksAndSigs(hashes [][]byte) *pb.DownloaderResponse {
	//TODO implement me
	panic("implement me")
}

func (c client) WaitForConnection(t time.Duration) bool {
	//TODO implement me
	panic("implement me")
}

func (c client) GetBlocksByHeights(heights []uint64) *pb.DownloaderResponse {
	//TODO implement me
	panic("implement me")
}

func (c client) PushNewBlock(selfPeerHash [20]byte, blockBytes []byte, timeout bool) (*pb.DownloaderResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (c client) GetPeers() (*pb.DownloaderResponse, error) {
	return c.response, c.err
}

func (c client) Close(reason string) {
	return
}

func (c client) IsReady() bool {
	return c.isReady
}
