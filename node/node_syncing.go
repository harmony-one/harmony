package node

import (
	"fmt"

	downloader_pb "github.com/harmony-one/harmony/api/service/syncing/downloader/proto"
)

// CalculateResponse implements DownloadInterface on Node object.
func (node *Node) CalculateResponse(
	request *downloader_pb.DownloaderRequest, incomingPeer string,
) (*downloader_pb.DownloaderResponse, error) {
	response := &downloader_pb.DownloaderResponse{}
	fmt.Println("something came in", request.String())
	return response, nil
}
