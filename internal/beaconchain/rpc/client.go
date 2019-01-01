package beaconchain

import (
	"context"
	"fmt"
	"log"
	"time"

	proto "github.com/harmony-one/harmony/api/beaconchain"

	"google.golang.org/grpc"
)

// Client is the client model for beaconchain service.
type Client struct {
	beaconChainServiceClient proto.BeaconChainServiceClient
	opts                     []grpc.DialOption
	conn                     *grpc.ClientConn
}

// NewClient setups a Client given ip and port.
func NewClient(ip, port string) *Client {
	client := Client{}
	client.opts = append(client.opts, grpc.WithInsecure())
	var err error
	client.conn, err = grpc.Dial(fmt.Sprintf("%s:%s", ip, port), client.opts...)
	if err != nil {
		log.Fatalf("fail to dial: %v", err)
		return nil
	}

	client.beaconChainServiceClient = proto.NewBeaconChainServiceClient(client.conn)
	return &client
}

// Close closes the Client.
func (client *Client) Close() {
	client.conn.Close()
}

// GetLeaders gets current leaders from beacon chain
func (client *Client) GetLeaders() *proto.FetchLeadersResponse {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	request := &proto.FetchLeadersRequest{}
	response, err := client.beaconChainServiceClient.FetchLeaders(ctx, request)
	if err != nil {
		log.Fatalf("Error fetching leaders from beacon chain: %s", err)
	}
	return response
}
