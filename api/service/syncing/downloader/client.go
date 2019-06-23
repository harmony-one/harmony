package downloader

import (
	"context"
	"fmt"
	"time"

	pb "github.com/harmony-one/harmony/api/service/syncing/downloader/proto"
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
	var err error
	client.conn, err = grpc.Dial(fmt.Sprintf(ip+":"+port), client.opts...)
	if err != nil {
		utils.GetLogInstance().Info("[SYNC] client.go:ClientSetup fail to dial: ", "IP", ip, "error", err)
		return nil
	}
	utils.GetLogInstance().Info("[SYNC] grpc connect successfully", "IP", ip)
	client.dlClient = pb.NewDownloaderClient(client.conn)
	return &client
}

// Close closes the Client.
func (client *Client) Close() {
	err := client.conn.Close()
	if err != nil {
		utils.GetLogInstance().Info("[SYNC] unable to close connection ")
	}
}

// GetBlockHashes gets block hashes from all the peers by calling grpc request.
func (client *Client) GetBlockHashes(startHash []byte, size uint32) *pb.DownloaderResponse {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	request := &pb.DownloaderRequest{Type: pb.DownloaderRequest_HEADER, BlockHash: startHash, Size: size}
	response, err := client.dlClient.Query(ctx, request)
	if err != nil {
		utils.GetLogInstance().Info("[SYNC] GetBlockHashes query failed", "error", err)
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
		utils.GetLogInstance().Info("[SYNC] downloader/client.go:GetBlocks query failed.", "error", err)
	}
	return response
}

// Register will register node's ip/port information to peers receive newly created blocks in future
// hash is the bytes of "ip:port" string representation
func (client *Client) Register(hash []byte, ip, port string) *pb.DownloaderResponse {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	request := &pb.DownloaderRequest{Type: pb.DownloaderRequest_REGISTER}
	request.PeerHash = make([]byte, len(hash))
	copy(request.PeerHash, hash)
	request.Ip = ip
	request.Port = port
	response, err := client.dlClient.Query(ctx, request)
	if err != nil || response == nil {
		utils.GetLogInstance().Info("[SYNC] client.go:Register failed.", "error", err, "response", response)
	}
	return response
}

// PushNewBlock will send the lastest verified block to registered nodes
func (client *Client) PushNewBlock(selfPeerHash [20]byte, blockHash []byte, timeout bool) *pb.DownloaderResponse {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	request := &pb.DownloaderRequest{Type: pb.DownloaderRequest_NEWBLOCK}
	request.BlockHash = make([]byte, len(blockHash))
	copy(request.BlockHash, blockHash)
	request.PeerHash = make([]byte, len(selfPeerHash))
	copy(request.PeerHash, selfPeerHash[:])

	if timeout {
		request.Type = pb.DownloaderRequest_REGISTERTIMEOUT
	}

	response, err := client.dlClient.Query(ctx, request)
	if err != nil {
		utils.GetLogInstance().Info("[SYNC] unable to send new block to unsync node", "error", err)
	}
	return response
}

// GetBlockChainHeight gets the blockheight from peer
func (client *Client) GetBlockChainHeight() *pb.DownloaderResponse {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	request := &pb.DownloaderRequest{Type: pb.DownloaderRequest_BLOCKHEIGHT}
	response, err := client.dlClient.Query(ctx, request)
	if err != nil {
		utils.GetLogInstance().Info("[SYNC] unable to get blockchain height", "error", err)
	}
	return response
}
