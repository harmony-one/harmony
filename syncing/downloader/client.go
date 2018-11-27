package downloader

import (
	"context"
	"fmt"
	"log"
	"time"

	pb "github.com/harmony-one/harmony/syncing/downloader/proto"
	"google.golang.org/grpc"
)

// Client ...
type Client struct {
	dlClient pb.DownloaderClient
	opts     []grpc.DialOption
	conn     *grpc.ClientConn
}

// ClientSetup ...
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

// Close ...
func (client *Client) Close() {
	client.conn.Close()
}

// GetBlockHashes ...
func (client *Client) GetBlockHashes() *pb.DownloaderResponse {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	request := &pb.DownloaderRequest{Type: pb.DownloaderRequest_HEADER}
	response, err := client.dlClient.Query(ctx, request)
	if err != nil {
		log.Fatalf("Error")
	}
	return response
}

// GetBlocks ...
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
		log.Fatalf("Error")
	}
	return response
}
