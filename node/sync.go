package node

import (
	"context"
	"fmt"
	"net"
	"time"

	downloader_pb "github.com/harmony-one/harmony/api/service/syncing/downloader/proto"
	"github.com/harmony-one/harmony/core/types"
	libp2p_peer "github.com/libp2p/go-libp2p-core/peer"
	manet "github.com/multiformats/go-multiaddr-net"
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
	ip := "127.0.0.1"
	port := defaultDownloadPort

	connection, _ := grpc.Dial(fmt.Sprintf(ip+":"+port), grpc.WithInsecure())

	syncingHandle := downloader_pb.NewDownloaderClient(connection)
	fmt.Println("here is a syncing handle", syncingHandle)
	// syncingHandle.Query(ctx.TODO(),
	// in *downloader_pb.DownloaderRequest, opts ...grpc.CallOption)
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
		coreAPI, _ := node.host.RawHandles()
		// addrs := ipfsNode.Peerstore.PeersWithAddrs()
		tick := time.NewTicker(time.Second * 10)
		defer tick.Stop()

		for range tick.C {
			conns, err := coreAPI.Swarm().Peers(context.TODO())

			if err != nil {
				return err
			}

			fmt.Println("ATTTTTTTTTTTTTTTTTT", len(conns))

			for _, conn := range conns {
				fmt.Println("from what to what", conn.Address())
				a, err := manet.ToNetAddr(conn.Address())

				if err != nil {
					return err
				}

				fmt.Println("nasty cast->", a.(*net.TCPAddr).Port)

				plainIP, err := manet.ToIP(conn.Address())
				fmt.Println("\nATTTTTN->", plainIP, "\n")

				if err != nil {
					return err
				}

				peer, err := libp2p_peer.AddrInfoFromP2pAddr(conn.Address())
				if err != nil {
					return err
				}

				fmt.Println("peeeeeeer", peer.String(), "and plain IP", plainIP)
				// panic("die ")
			}

		}
		return nil
	})

	return g.Wait()
}
