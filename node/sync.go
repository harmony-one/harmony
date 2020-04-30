package node

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"

	downloader_pb "github.com/harmony-one/harmony/api/service/syncing/downloader/proto"
	"github.com/harmony-one/harmony/core/types"
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

func (s *simpleSyncer) Query(
	ctx context.Context, request *downloader_pb.DownloaderRequest,
) (*downloader_pb.DownloaderResponse, error) {

	fmt.Println("called in query")
	response := &downloader_pb.DownloaderResponse{}
	blk := <-s.currentBlockHeight
	response.BlockHeight = blk.NumberU64()

	fmt.Println("sending out", response.String(), request.String())
	return response, nil
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

		coreAPI, _ := node.host.RawHandles()
		tick := time.NewTicker(time.Second * 10)

		defer tick.Stop()
		// NOTE while coding it, do return err, later do continue
		for range tick.C {
			conns, err := coreAPI.Swarm().Peers(context.TODO())

			if err != nil {
				return err
			}

			var collect sync.WaitGroup

			for _, conn := range conns {
				_, ip, err := manet.DialArgs(conn.Address())
				if err != nil {
					return err
				}
				peerID := conn.ID().ShortString()
				time.Sleep(1 * time.Second)
				client, err := lookupClient(peerID, ip)
				if err != nil {
					fmt.Println("died here but will continue", err.Error())
					continue
				}

				collect.Add(1)
				go func() {
					defer collect.Done()
					resp, err := client.askHeight()
				}()

				fmt.Println("cant believe i got a response", resp, err)
			}

			collect.Wait()

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
