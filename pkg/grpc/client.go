package grpc

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	proto "github.com/vega-project/ccb-operator/proto"
)

type Client interface {
	StoreData(parameters map[string]string, results string) (*proto.StoreResponse, error)
	GetData(parameters map[string]string) (*proto.GetDataResponse, error)
	Close() error
}

type client struct {
	client proto.DbServiceClient
	conn   *grpc.ClientConn
}

// NewClient creates a new Client connected to the given address.
func NewClient(address string) (Client, error) {
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	return &client{client: proto.NewDbServiceClient(conn), conn: conn}, nil
}

// StoreData stores the given data in the gRPC server.
func (c *client) StoreData(parameters map[string]string, results string) (*proto.StoreResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return c.client.StoreData(ctx, &proto.StoreRequest{
		Parameters: parameters,
		Results:    results,
	})
}

// GetData retrieves the data from the database filtered by the given parameters.
func (c *client) GetData(parameters map[string]string) (*proto.GetDataResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return c.client.GetData(ctx, &proto.GetDataRequest{Parameters: parameters})
}

// Close closes the connection to the gRPC server.
func (c *client) Close() error {
	return c.conn.Close()
}
