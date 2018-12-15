package client

import (
	"context"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	proto "github.com/harmony-one/harmony/client/service/proto"
	"log"
	"time"

	"google.golang.org/grpc"
)

// Client is the client model for client service.
type Client struct {
	clientServiceClient proto.ClientServiceClient
	opts                []grpc.DialOption
	conn                *grpc.ClientConn
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

	client.clientServiceClient = proto.NewClientServiceClient(client.conn)
	return &client
}

// Close closes the Client.
func (client *Client) Close() {
	client.conn.Close()
}

// GetBalance gets account balance from the client service.
func (client *Client) GetBalance(address common.Address) *proto.FetchAccountStateResponse {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	request := &proto.FetchAccountStateRequest{Address: address.Bytes()}
	response, err := client.clientServiceClient.FetchAccountState(ctx, request)
	if err != nil {
		log.Fatalf("Error getting balance: %s", err)
	}
	return response
}

// GetFreeToken requests free token from the faucet contract.
func (client *Client) GetFreeToken(address common.Address) *proto.GetFreeTokenResponse {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	request := &proto.GetFreeTokenRequest{Address: address.Bytes()}
	response, err := client.clientServiceClient.GetFreeToken(ctx, request)
	if err != nil {
		log.Fatalf("Error getting free token: %s", err)
	}
	return response
}
