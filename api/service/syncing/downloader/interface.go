package downloader

import (
	pb "github.com/harmony-one/harmony/api/service/syncing/downloader/proto"
)

// DownloadInterface is the interface for downloader package.
type DownloadInterface interface {
	// Syncing blockchain from other peers.
	// The returned channel is the signal of syncing finish.
	CalculateResponse(request *pb.DownloaderRequest) (*pb.DownloaderResponse, error)
}
