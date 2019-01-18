package downloader

import (
	"context"
	"fmt"
	"log"
	"time"

	pb "github.com/harmony-one/harmony/api/services/syncing/downloader/proto"
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
	client.conn, err = grpc.Dial(fmt.Sprintf("%s:%s", ip, port), client.opts...)
	if err != nil {
		log.Fatalf("fail to dial: %v", err)
		return nil
	}

	client.dlClient = pb.NewDownloaderClient(client.conn)
	return &client
}

// Close closes the Client.
func (client *Client) Close() {
	client.conn.Close()
}

// GetBlockHashes gets block hashes from all the peers by calling grpc request.
func (client *Client) GetBlockHashes(startHash []byte) *pb.DownloaderResponse {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	request := &pb.DownloaderRequest{Type: pb.DownloaderRequest_HEADER}
	request.BlockHash = startHash
	response, err := client.dlClient.Query(ctx, request)
	if err != nil {
		log.Printf("[sync] downloader/client.go:GetBlockHashes query failed")
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
		log.Printf("[sync] downloader/client.go:GetBlocks query failed")
	}
	return response
}

// Register will register node's ip/port information to peers receive newly created blocks in future
// hash is the bytes of "ip:port" string representation
func (client *Client) Register(hash []byte) *pb.DownloaderResponse {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	request := &pb.DownloaderRequest{Type: pb.DownloaderRequest_REGISTER}
	request.PeerHash = make([]byte, len(hash))
	copy(request.PeerHash, hash)
	response, err := client.dlClient.Query(ctx, request)
	if err != nil {
		log.Printf("[sync] client.go:Register error", "code", err)
	}
	return response
}

func (client *Client) PushNewBlock(peerHash []byte, blockHash []byte, timeout bool) *pb.DownloaderResponse {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	request := &pb.DownloaderRequest{Type: pb.DownloaderRequest_NEWBLOCK}
	request.BlockHash = make([]byte, len(blockHash))
	copy(request.BlockHash, blockHash)
	request.PeerHash = make([]byte, len(peerHash))
	copy(request.PeerHash, peerHash)

	if timeout {
		request.Type = pb.DownloaderRequest_REGISTERTIMEOUT
	}

	response, err := client.dlClient.Query(ctx, request)
	log.Printf("[sync] response from pushnewblock", "response", response)
	if err != nil {
		log.Printf("[sync] unable to send new block to unsync node with error: %v", err)
	}
	return response
}
