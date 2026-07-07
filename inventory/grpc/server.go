package grpc

import (
	"context"
	"log/slog"

	"coursework/inventory/domain"
	inventoryv1 "coursework/proto/gen/inventory/v1"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Server adapts domain.ReservationService to the generated
// InventoryServiceServer interface.
type Server struct {
	inventoryv1.UnimplementedInventoryServiceServer
	service *domain.ReservationService
	logger  *slog.Logger
}

func NewServer(service *domain.ReservationService, logger *slog.Logger) *Server {
	return &Server{service: service, logger: logger}
}

func (s *Server) Reserve(ctx context.Context, req *inventoryv1.ReserveRequest) (*inventoryv1.ReserveResponse, error) {
	if req.GetSku() == "" {
		return nil, status.Error(codes.InvalidArgument, "sku is required")
	}
	result, err := s.service.Reserve(ctx, req.GetSku(), int(req.GetQuantity()))
	if err != nil {
		s.logger.ErrorContext(ctx, "reserve failed", "sku", req.GetSku(), "error", err)
		return nil, status.Errorf(codes.Internal, "reserve failed: %v", err)
	}
	return &inventoryv1.ReserveResponse{Success: result.Success, Reason: result.Reason, Attempts: int32(result.Attempts)}, nil
}

func (s *Server) Release(ctx context.Context, req *inventoryv1.ReleaseRequest) (*inventoryv1.ReleaseResponse, error) {
	if req.GetSku() == "" {
		return nil, status.Error(codes.InvalidArgument, "sku is required")
	}
	result, err := s.service.Release(ctx, req.GetSku(), int(req.GetQuantity()))
	if err != nil {
		s.logger.ErrorContext(ctx, "release failed", "sku", req.GetSku(), "error", err)
		return nil, status.Errorf(codes.Internal, "release failed: %v", err)
	}
	return &inventoryv1.ReleaseResponse{Success: result.Success, Reason: result.Reason, Attempts: int32(result.Attempts)}, nil
}
