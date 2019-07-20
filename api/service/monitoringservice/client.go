package monitoringservice

import (
	"context"
	"fmt"
	"log"
	"time"

	"google.golang.org/grpc"
)

// Client is the client model for client service.
type Client struct {
	clientServiceClient ClientServiceClient
	opts                []grpc.DialOption
	conn                *grpc.ClientConn
}

// NewClient setups a Client given ip and port.
func NewClient(ip string) *Client {
	client := Client{}
	client.opts = append(client.opts, grpc.WithInsecure())
	var err error
	client.conn, err = grpc.Dial(fmt.Sprintf("%s:%s", ip, Port), client.opts...)
	if err != nil {
		log.Fatalf("fail to dial: %v", err)
		return nil
	}

	client.clientServiceClient = NewClientServiceClient(client.conn)
	return &client
}

// Close closes the Client.
func (client *Client) Close() error {
	return client.conn.Close()
}

// Process processes request.
func (client *Client) Process(request *Request) (*Response, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	response, err := client.clientServiceClient.Process(ctx, request)
	return response, err
}
