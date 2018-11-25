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

// GetHeaders ...
func (client *Client) GetHeaders() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	request := &pb.DownloaderRequest{Type: pb.DownloaderRequest_HEADER}

	result, err := client.dlClient.Query(ctx, request)
	if err != nil {
		log.Fatalf("%v.GetFeatures(_) = _, %v: ", client, err)
	}
	log.Println(result)
}

// Close ...
func (client *Client) Close() {
	client.conn.Close()
}

// ClientSetUp ...
func ClientSetUp(ip, port string) *Client {
	client := Client{}
	var err error
	client.conn, err = grpc.Dial(fmt.Sprintf("%s:%s", ip, port), client.opts...)
	if err != nil {
		log.Fatalf("fail to dial: %v", err)
	}
	client.dlClient = pb.NewDownloaderClient(client.conn)
	return &client
}
