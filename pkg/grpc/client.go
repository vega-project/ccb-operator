package grpc

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	proto "github.com/vega-project/ccb-operator/proto"
)

type Client struct {
	client proto.DbServiceClient
	conn   *grpc.ClientConn
}

// NewClient creates a new Client connected to the given address.
func NewClient(address string) (*Client, error) {
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	return &Client{client: proto.NewDbServiceClient(conn), conn: conn}, nil
}

// StoreData stores the given data in the gRPC server.
func (c *Client) StoreData(parametres map[string]string, results string) (*proto.StoreResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return c.client.StoreData(ctx, &proto.StoreRequest{
		Parameters: parametres,
		Results:    results,
	})
}

// GetData retrieves the data from the database filtered by the given parameters.
func (c *Client) GetData(parametres map[string]string) (*proto.GetDataResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return c.client.GetData(ctx, &proto.GetDataRequest{Parameters: parametres})
}

// Close closes the connection to the gRPC server.
func (c *Client) Close() error {
	return c.conn.Close()
}
