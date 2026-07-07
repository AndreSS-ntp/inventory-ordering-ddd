package grpc

import (
	"context"
	"time"

	inventoryv1 "coursework/proto/gen/inventory/v1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type InventoryClient struct {
	client  inventoryv1.InventoryServiceClient
	timeout time.Duration
}

func NewInventoryClient(addr string, timeout time.Duration) (*InventoryClient, func() error, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, nil, err
	}
	return &InventoryClient{client: inventoryv1.NewInventoryServiceClient(conn), timeout: timeout}, conn.Close, nil
}

func (c *InventoryClient) Reserve(ctx context.Context, sku string, quantity int) (bool, string, int, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	resp, err := c.client.Reserve(ctx, &inventoryv1.ReserveRequest{Sku: sku, Quantity: int32(quantity)})
	if err != nil {
		return false, "", 0, err
	}
	return resp.GetSuccess(), resp.GetReason(), int(resp.GetAttempts()), nil
}
