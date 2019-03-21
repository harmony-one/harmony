package downloader

import (
	pb "github.com/harmony-one/harmony/api/service/syncing/downloader/proto"
)

// DownloadInterface is the interface for downloader package.
type DownloadInterface interface {
	// State Syncing server-side interface, responsible for all kinds of state syncing grpc calls
	CalculateResponse(request *pb.DownloaderRequest) (*pb.DownloaderResponse, error)
}
