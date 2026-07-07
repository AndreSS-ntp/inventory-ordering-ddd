package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"coursework/inventory/domain"
	invgrpc "coursework/inventory/grpc"
	"coursework/inventory/repository"
	inventoryv1 "coursework/proto/gen/inventory/v1"

	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	dsn := getenv("INVENTORY_DB_DSN", "postgres://postgres:postgres@localhost:5433/inventory_db?sslmode=disable")
	grpcPort := getenv("INVENTORY_GRPC_PORT", "50052")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool, err := connectWithRetry(ctx, dsn, logger)
	if err != nil {
		logger.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	repo := repository.NewPostgresStockRepository(pool)
	service := domain.NewReservationService(repo, logger)
	server := invgrpc.NewServer(service, logger)

	lis, err := net.Listen("tcp", ":"+grpcPort)
	if err != nil {
		logger.Error("failed to listen", "port", grpcPort, "error", err)
		os.Exit(1)
	}

	grpcServer := grpc.NewServer()
	inventoryv1.RegisterInventoryServiceServer(grpcServer, server)

	healthServer := health.NewServer()
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
	grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)

	reflection.Register(grpcServer)

	go func() {
		logger.Info("inventory grpc server listening", "port", grpcPort)
		if err := grpcServer.Serve(lis); err != nil {
			logger.Error("grpc server stopped", "error", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	logger.Info("shutting down inventory server")
	grpcServer.GracefulStop()
}

func connectWithRetry(ctx context.Context, dsn string, logger *slog.Logger) (*pgxpool.Pool, error) {
	const maxAttempts = 10
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		pool, err := pgxpool.New(ctx, dsn)
		if err == nil {
			if pingErr := pool.Ping(ctx); pingErr == nil {
				return pool, nil
			} else {
				lastErr = pingErr
				pool.Close()
			}
		} else {
			lastErr = err
		}
		logger.Warn("database not ready yet, retrying", "attempt", attempt, "max_attempts", maxAttempts, "error", lastErr)
		select {
		case <-time.After(time.Duration(attempt) * 500 * time.Millisecond):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return nil, errors.New("could not connect to database: " + lastErr.Error())
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
