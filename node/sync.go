package node

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"time"

	downloader_pb "github.com/harmony-one/harmony/api/service/syncing/downloader/proto"
	"github.com/harmony-one/harmony/core/types"
	ipfs_interface "github.com/ipfs/interface-go-ipfs-core"
	manet "github.com/multiformats/go-multiaddr-net"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/singleflight"
	"google.golang.org/grpc"
)

var clients singleflight.Group

func lookupClient(peerID string, ip string) (*grpcClientWrapper, error) {
	handle, err, _ := clients.Do(
		peerID, func() (interface{}, error) {

			time.AfterFunc(time.Minute*10, func() {
				clients.Forget(peerID)
			})

			host, port, err := net.SplitHostPort(ip)
			if err != nil {
				return nil, err
			}

			if host != "127.0.0.1" {
				return nil, errors.Errorf("was not a localhost %s", host)
			}

			otherSide := host + ":" + offSetSyncingPort(port)
			connection, err := grpc.Dial(otherSide, grpc.WithInsecure())
			if err != nil {
				return nil, err
			}

			return &grpcClientWrapper{
				DownloaderClient: downloader_pb.NewDownloaderClient(connection),
			}, nil
		},
	)

	if err != nil {
		return nil, err
	}

	return handle.(*grpcClientWrapper), nil
}

type simpleSyncer struct {
	currentBlockHeight chan *types.Block
}

type grpcClientWrapper struct {
	downloader_pb.DownloaderClient
}

var (
	askBlockHeight = &downloader_pb.DownloaderRequest{
		Type: downloader_pb.DownloaderRequest_BLOCKHEIGHT,
	}
)

func (c *grpcClientWrapper) askHeight() (*downloader_pb.DownloaderResponse, error) {
	return c.Query(context.TODO(), askBlockHeight, grpc.WaitForReady(true))
}

// NOTE maybe better named handle incoming request
func (s *simpleSyncer) Query(
	ctx context.Context, request *downloader_pb.DownloaderRequest,
) (*downloader_pb.DownloaderResponse, error) {

	switch request.GetType() {
	case downloader_pb.DownloaderRequest_BLOCKHASH:
	case downloader_pb.DownloaderRequest_BLOCK:
	case downloader_pb.DownloaderRequest_NEWBLOCK:
	case downloader_pb.DownloaderRequest_BLOCKHEIGHT:
	case downloader_pb.DownloaderRequest_REGISTER:
	case downloader_pb.DownloaderRequest_REGISTERTIMEOUT:
	case downloader_pb.DownloaderRequest_UNKNOWN:
	case downloader_pb.DownloaderRequest_BLOCKHEADER:

	}

	response := &downloader_pb.DownloaderResponse{}
	blk := <-s.currentBlockHeight
	response.BlockHeight = blk.NumberU64()
	return response, nil
}

type r struct {
	result interface{}
	err    error
}

func heightOfPeers(conns []ipfs_interface.ConnectionInfo) {
	var collect errgroup.Group

	results := make(chan r, len(conns))

	for _, conn := range conns {
		collect.Go(func() error {

			_, ip, err := manet.DialArgs(conn.Address())
			if err != nil {
				go func() {
					results <- r{nil, err}
				}()
				return err
			}
			peerID := conn.ID().ShortString()
			time.Sleep(1 * time.Second)
			client, err := lookupClient(peerID, ip)

			if err != nil {
				go func() {
					results <- r{nil, err}
				}()
				return err
			}

			resp, err := client.askHeight()
			if err != nil {
				go func() {
					results <- r{nil, err}
				}()
				return err
			}
			go func() {
				results <- r{resp, nil}
			}()
			return nil
		})
	}

	if firstErr := collect.Wait(); firstErr != nil {
		fmt.Println("here is the first error from whole group", firstErr.Error())
	}

	// drain the channel
	for i := 0; i < len(conns); i++ {
		syncResult := <-results
		if e := syncResult.err; e != nil {
			fmt.Println("sync result had problem", e.Error())
		} else {
			fmt.Println("sync result was:", syncResult.result)
		}
	}

}

// StartStateSync ..
func (node *Node) StartStateSync() error {
	addr := net.JoinHostPort("", offSetSyncingPort(node.Peer.Port))
	lis, err := net.Listen("tcp4", addr)

	if err != nil {
		return err
	}

	var g errgroup.Group

	in := make(chan *types.Block, 1)
	// beacon := make(chan *types.Block)

	simple := &simpleSyncer{
		currentBlockHeight: in,
	}

	go func() {
		go func() {
			for {
				in <- node.Blockchain().CurrentBlock()
			}
		}()

		// if node.Consensus.ShardID == shard.BeaconChainShardID {
		// 	go func() {
		// 		for {
		// 			in <- node.Beaconchain().CurrentBlock()
		// 		}
		// 	}()
		// }

	}()

	grpcServer := grpc.NewServer()
	downloader_pb.RegisterDownloaderServer(grpcServer, simple)

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
		coreAPI := node.host.CoreAPI
		tick := time.NewTicker(time.Second * 10)
		defer tick.Stop()

		for range tick.C {
			conns, err := coreAPI.Swarm().Peers(context.TODO())

			if err != nil {
				return err
			}

			heightOfPeers(conns)
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
