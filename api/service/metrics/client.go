package metrics

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/harmony-one/harmony/internal/utils"
	"google.golang.org/grpc"
)

// Constants for client service.
const (
	metricsGrpcClientPortDifference = 29000
)

// Client is the client model for client service.
type Client struct {
	clientServiceClient ClientServiceClient
	opts                []grpc.DialOption
	conn                *grpc.ClientConn
}

// GetMetricsGrpcClientPort returns the grpc client service port from node port.
func GetMetricsGrpcClientPort(nodePort string) string {
	if port, err := strconv.Atoi(nodePort); err == nil {
		return fmt.Sprintf("%d", port-metricsGrpcClientPortDifference)
	}
	utils.Logger().Error().Msg("error on parsing.")
	return ""
}

// NewClient setups a Client given ip and port.
func NewClient(ip, port string) *Client {
	client := Client{}
	client.opts = append(client.opts, grpc.WithInsecure())
	var err error
	client.conn, err = grpc.Dial(fmt.Sprintf("%s:%s", ip, GetMetricsGrpcClientPort(port)), client.opts...)
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
