package node

import (
	"context"
	"fmt"
	"net"

	downloader_pb "github.com/harmony-one/harmony/api/service/syncing/downloader/proto"
	"github.com/harmony-one/harmony/core/types"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
)

const (
	defaultDownloadPort = "6666"
)

type simpleSyncer struct {
}

// CalculateResponse implements DownloadInterface on Node object.
func (s *simpleSyncer) CalculateResponse(
	request *downloader_pb.DownloaderRequest, incomingPeer string,
) (*downloader_pb.DownloaderResponse, error) {
	response := &downloader_pb.DownloaderResponse{}
	fmt.Println("something came in", request.String())
	return response, nil
}

func (s *simpleSyncer) Query(
	ctx context.Context, request *downloader_pb.DownloaderRequest,
) (*downloader_pb.DownloaderResponse, error) {

	response := &downloader_pb.DownloaderResponse{}

	fmt.Println("called in query")

	return response, nil
	// var pinfo string
	// // retrieve ip/port information; used for debug only
	// p, ok := peer.FromContext(ctx)
	// if !ok {
	// 	pinfo = ""
	// } else {
	// 	pinfo = p.Addr.String()
	// }

	// // fmt.Println("ask around", request.String(), pinfo)

	// response, err := s.downloadInterface.CalculateResponse(request, pinfo)

	// // fmt.Println("reply->", response.String(), err)
	// if err != nil {
	// 	return nil, err
	// }
	// return response, nil
}

// StartStateSync ..
func (node *Node) StartStateSync() error {
	addr := net.JoinHostPort("", defaultDownloadPort)
	lis, err := net.Listen("tcp4", addr)
	if err != nil {
		return err
	}

	in := make(chan *types.Block)

	simple := &simpleSyncer{}
	grpcServer := grpc.NewServer()

	downloader_pb.RegisterDownloaderServer(grpcServer, simple)

	var g errgroup.Group

	g.Go(func() error {
		return grpcServer.Serve(lis)
	})

	g.Go(func() error {
		for beaconBlock := range in {
			_ = beaconBlock
		}
		return nil
	})

	g.Go(func() error {
		for ownChain := range in {
			_ = ownChain
		}
		return nil
	})

	return g.Wait()
}
