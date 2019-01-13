package downloader

import (
	pb "github.com/harmony-one/harmony/api/services/syncing/downloader/proto"
)

// DownloadInterface is the interface for downloader package.
type DownloadInterface interface {
	// Syncing blockchain from other peers.
	// The returned channel is the signal of syncing finish.
	// peerInfoStr is the string representaion of the peer ip and port to identify the peer used to register out of sync nodes
	CalculateResponse(request *pb.DownloaderRequest) (*pb.DownloaderResponse, error)
}
