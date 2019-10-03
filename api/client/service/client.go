package client

import (
	"context"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/common"
	proto "github.com/harmony-one/harmony/api/client/service/proto"

	"google.golang.org/grpc"
)

// Client is the client model for client service.
type Client struct {
	clientServiceClient proto.ClientServiceClient
	opts                []grpc.DialOption
	conn                *grpc.ClientConn
}

// NewClient setups a Client given ip and port.
func NewClient(ip, port string) (*Client, error) {
	client := Client{}
	client.opts = append(client.opts, grpc.WithInsecure())
	var err error
	client.conn, err = grpc.Dial(fmt.Sprintf("%s:%s", ip, port), client.opts...)
	if err != nil {
		return nil, err
	}

	client.clientServiceClient = proto.NewClientServiceClient(client.conn)
	return &client, nil
}

// Close closes the Client.
func (client *Client) Close() error {
	return client.conn.Close()
}

// GetBalance gets account balance from the client service.
func (client *Client) GetBalance(address common.Address) (*proto.FetchAccountStateResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	request := &proto.FetchAccountStateRequest{Address: address.Bytes()}
	return client.clientServiceClient.FetchAccountState(ctx, request)
}

// GetFreeToken requests free token from the faucet contract.
func (client *Client) GetFreeToken(address common.Address) (*proto.GetFreeTokenResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	request := &proto.GetFreeTokenRequest{Address: address.Bytes()}
	return client.clientServiceClient.GetFreeToken(ctx, request)
}
