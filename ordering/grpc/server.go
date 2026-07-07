package grpc

import (
	"context"
	"errors"
	"log/slog"

	"coursework/ordering/domain"
	orderingv1 "coursework/proto/gen/ordering/v1"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Server struct {
	orderingv1.UnimplementedOrderingServiceServer
	service *domain.OrderService
	logger  *slog.Logger
}

func NewServer(service *domain.OrderService, logger *slog.Logger) *Server {
	return &Server{service: service, logger: logger}
}

func (s *Server) CreateOrder(ctx context.Context, req *orderingv1.CreateOrderRequest) (*orderingv1.Order, error) {
	order, _, err := s.service.PlaceOrder(ctx, req.GetSku(), int(req.GetQuantity()))
	if err != nil {
		if errors.Is(err, domain.ErrInvalidQuantity) {
			return nil, status.Error(codes.InvalidArgument, "quantity must be a positive integer")
		}
		s.logger.ErrorContext(ctx, "create order failed", "error", err)
		return nil, status.Errorf(codes.Internal, "create order failed: %v", err)
	}

	return &orderingv1.Order{
		Id:            order.ID,
		Sku:           order.SKU,
		Quantity:      int32(order.Quantity),
		Status:        string(order.Status),
		FailureReason: order.FailureReason,
		CreatedAt:     order.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}, nil
}
