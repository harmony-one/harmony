package node

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"time"

	downloader_pb "github.com/harmony-one/harmony/api/service/syncing/downloader/proto"
	"github.com/harmony-one/harmony/core/types"
	manet "github.com/multiformats/go-multiaddr-net"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/singleflight"
	"google.golang.org/grpc"
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
	addr := net.JoinHostPort("", offSetSyncingPort(node.Peer.Port))
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

	g.Go(func() error {
		var clients singleflight.Group

		coreAPI, _ := node.host.RawHandles()
		tick := time.NewTicker(time.Second * 10)

		defer tick.Stop()
		// NOTE while coding it, do return err, later do continue
		for range tick.C {
			conns, err := coreAPI.Swarm().Peers(context.TODO())

			if err != nil {
				return err
			}

			fmt.Println("how many swarm connections?", len(conns))

			for _, conn := range conns {

				_, ip, err := manet.DialArgs(conn.Address())
				if err != nil {
					return err
				}

				peerID := conn.ID().ShortString()

				handle, err, _ := clients.Do(
					peerID, func() (interface{}, error) {

						time.AfterFunc(time.Minute*10, func() {
							clients.Forget(peerID)
						})

						host, port, err := net.SplitHostPort(ip)
						if err != nil {
							return nil, err
						}

						fmt.Println("ripped the dial args", host, port, err)

						if host != "127.0.0.1" {
							return nil, errors.Errorf("was not a localhost %s", host)
						}

						otherSide := host + ":" + offSetSyncingPort(port)
						fmt.Println("gonna try to talk to ", otherSide)
						connection, err := grpc.Dial(otherSide, grpc.WithInsecure())
						if err != nil {
							return nil, err
						}

						return downloader_pb.NewDownloaderClient(connection), nil
					},
				)

				if err != nil {
					fmt.Println("died here but will continue", err.Error())
					continue
				}

				client := handle.(downloader_pb.DownloaderClient)
				resp, err := client.Query(
					context.TODO(),
					&downloader_pb.DownloaderRequest{},
				)

				fmt.Println("cant believe i got a response", resp, err)

			}

		}
		return nil
	})

	return g.Wait()
}

func offSetSyncingPort(nodePort string) string {
	if port, err := strconv.Atoi(nodePort); err == nil {
		return fmt.Sprintf("%d", port-syncingPortDifference)
	}
	return ""
}

const (
	syncingPortDifference = 3000
)
