package downloader

import (
	pb "github.com/harmony-one/harmony/api/service/legacysync/downloader/proto"
)

// DownloadInterface is the interface for downloader package.
type DownloadInterface interface {
	// State Syncing server-side interface, responsible for all kinds of state syncing grpc calls
	// incomingPeer is incoming peer ip:port information
	CalculateResponse(request *pb.DownloaderRequest, incomingPeer string) (*pb.DownloaderResponse, error)
}
