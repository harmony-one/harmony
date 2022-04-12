package downloader

import (
	"context"
	"fmt"
	"time"

	pb "github.com/harmony-one/harmony/api/service/legacysync/downloader/proto"
	"github.com/harmony-one/harmony/internal/utils"
	"google.golang.org/grpc"
)

// Client is the client model for downloader package.
type Client struct {
	dlClient pb.DownloaderClient
	opts     []grpc.DialOption
	conn     *grpc.ClientConn
}

// ClientSetup setups a Client given ip and port.
func ClientSetup(ip, port string) *Client {
	client := Client{}
	client.opts = append(client.opts, grpc.WithInsecure())
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var err error
	client.conn, err = grpc.DialContext(ctx, fmt.Sprintf(ip+":"+port), client.opts...)
	if err != nil {
		utils.Logger().Error().Err(err).Str("ip", ip).Msg("[SYNC] client.go:ClientSetup fail to dial")
		return nil
	}
	utils.Logger().Debug().Str("ip", ip).Msg("[SYNC] grpc connect successfully")
	client.dlClient = pb.NewDownloaderClient(client.conn)
	return &client
}

// Close closes the Client.
func (client *Client) Close() {
	err := client.conn.Close()
	if err != nil {
		utils.Logger().Info().Msg("[SYNC] unable to close connection")
	}
}

// GetBlockHashes gets block hashes from all the peers by calling grpc request.
func (client *Client) GetBlockHashes(startHash []byte, size uint32, ip, port string) *pb.DownloaderResponse {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	request := &pb.DownloaderRequest{Type: pb.DownloaderRequest_BLOCKHASH, BlockHash: startHash, Size: size}
	request.Ip = ip
	request.Port = port
	response, err := client.dlClient.Query(ctx, request)
	if err != nil {
		utils.Logger().Error().Err(err).Str("target", client.conn.Target()).Msg("[SYNC] GetBlockHashes query failed")
	}
	return response
}

// GetBlocksByHeights gets blocks from peers by calling grpc request.
func (client *Client) GetBlocksByHeights(heights []uint64) *pb.DownloaderResponse {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	request := &pb.DownloaderRequest{
		Type:    pb.DownloaderRequest_BLOCKBYHEIGHT,
		Heights: heights,
	}
	response, err := client.dlClient.Query(ctx, request)
	if err != nil {
		utils.Logger().Error().Err(err).Str("target", client.conn.Target()).Msg("[SYNC] GetBlockHashes query failed")
	}
	return response
}

// GetBlockHeaders gets block headers in serialization byte array by calling a grpc request.
func (client *Client) GetBlockHeaders(hashes [][]byte) *pb.DownloaderResponse {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	request := &pb.DownloaderRequest{Type: pb.DownloaderRequest_BLOCKHEADER}
	request.Hashes = make([][]byte, len(hashes))
	for i := range hashes {
		request.Hashes[i] = make([]byte, len(hashes[i]))
		copy(request.Hashes[i], hashes[i])
	}
	response, err := client.dlClient.Query(ctx, request)
	if err != nil {
		utils.Logger().Error().Err(err).Str("target", client.conn.Target()).Msg("[SYNC] downloader/client.go:GetBlockHeaders query failed")
	}
	return response
}

// GetBlocks gets blocks in serialization byte array by calling a grpc request.
func (client *Client) GetBlocks(hashes [][]byte) *pb.DownloaderResponse {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	request := &pb.DownloaderRequest{Type: pb.DownloaderRequest_BLOCK}
	request.Hashes = make([][]byte, len(hashes))
	for i := range hashes {
		request.Hashes[i] = make([]byte, len(hashes[i]))
		copy(request.Hashes[i], hashes[i])
	}
	response, err := client.dlClient.Query(ctx, request)
	if err != nil {
		utils.Logger().Error().Err(err).Str("target", client.conn.Target()).Msg("[SYNC] downloader/client.go:GetBlocks query failed")
	}
	return response
}

// GetBlocksAndSigs get blockWithSig in serialization byte array by calling a grpc request
func (client *Client) GetBlocksAndSigs(hashes [][]byte) *pb.DownloaderResponse {
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	request := &pb.DownloaderRequest{Type: pb.DownloaderRequest_BLOCK, GetBlocksWithSig: true}
	request.Hashes = make([][]byte, len(hashes))
	for i := range hashes {
		request.Hashes[i] = make([]byte, len(hashes[i]))
		copy(request.Hashes[i], hashes[i])
	}
	response, err := client.dlClient.Query(ctx, request)
	if err != nil {
		utils.Logger().Error().Err(err).Str("target", client.conn.Target()).Msg("[SYNC] downloader/client.go:GetBlocksAndSigs query failed")
	}
	return response
}

// Register will register node's ip/port information to peers receive newly created blocks in future
// hash is the bytes of "ip:port" string representation
func (client *Client) Register(hash []byte, ip, port string) *pb.DownloaderResponse {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	request := &pb.DownloaderRequest{Type: pb.DownloaderRequest_REGISTER, RegisterWithSig: true}
	request.PeerHash = make([]byte, len(hash))
	copy(request.PeerHash, hash)
	request.Ip = ip
	request.Port = port
	response, err := client.dlClient.Query(ctx, request)
	if err != nil || response == nil {
		utils.Logger().Error().Err(err).Str("target", client.conn.Target()).Interface("response", response).Msg("[SYNC] client.go:Register failed")
	}
	return response
}

// PushNewBlock will send the lastest verified block to registered nodes
func (client *Client) PushNewBlock(selfPeerHash [20]byte, blockBytes []byte, timeout bool) (*pb.DownloaderResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	request := &pb.DownloaderRequest{Type: pb.DownloaderRequest_NEWBLOCK}
	request.BlockHash = make([]byte, len(blockBytes))
	copy(request.BlockHash, blockBytes)
	request.PeerHash = make([]byte, len(selfPeerHash))
	copy(request.PeerHash, selfPeerHash[:])

	if timeout {
		request.Type = pb.DownloaderRequest_REGISTERTIMEOUT
	}

	response, err := client.dlClient.Query(ctx, request)
	if err != nil {
		utils.Logger().Error().Err(err).Str("target", client.conn.Target()).Msg("[SYNC] unable to send new block to unsync node")
	}
	return response, err
}

// GetBlockChainHeight gets the blockheight from peer
func (client *Client) GetBlockChainHeight() (*pb.DownloaderResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	request := &pb.DownloaderRequest{Type: pb.DownloaderRequest_BLOCKHEIGHT}
	response, err := client.dlClient.Query(ctx, request)
	if err != nil {
		return nil, err
	}
	return response, nil
}
