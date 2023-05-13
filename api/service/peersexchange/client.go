package peersexchange

import (
	pb "github.com/harmony-one/harmony/api/service/legacysync/downloader/proto"
)

type Client interface {
	IsReady() bool
	GetPeers() (*pb.DownloaderResponse, error)
	Close(reason string)
}
