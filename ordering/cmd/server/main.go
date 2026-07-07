package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"coursework/ordering/domain"
	ordgrpc "coursework/ordering/grpc"
	"coursework/ordering/httpapi"
	"coursework/ordering/repository"
	orderingv1 "coursework/proto/gen/ordering/v1"

	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	dsn := getenv("ORDERING_DB_DSN", "postgres://postgres:postgres@localhost:5434/ordering_db?sslmode=disable")
	grpcPort := getenv("ORDERING_GRPC_PORT", "50051")
	httpPort := getenv("ORDERING_HTTP_PORT", "8080")
	inventoryAddr := getenv("INVENTORY_GRPC_ADDR", "localhost:50052")
	inventoryTimeout := getDurationEnv("INVENTORY_CALL_TIMEOUT", 3*time.Second)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool, err := connectWithRetry(ctx, dsn, logger)
	if err != nil {
		logger.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	inventoryClient, closeInventory, err := ordgrpc.NewInventoryClient(inventoryAddr, inventoryTimeout)
	if err != nil {
		logger.Error("failed to create inventory client", "error", err)
		os.Exit(1)
	}
	defer closeInventory()

	repo := repository.NewPostgresOrderRepository(pool)
	service := domain.NewOrderService(repo, inventoryClient, logger)

	httpServer := &http.Server{
		Addr:    ":" + httpPort,
		Handler: httpapi.NewRouter(httpapi.NewHandler(service, logger)),
	}

	grpcServer := grpc.NewServer()
	orderingv1.RegisterOrderingServiceServer(grpcServer, ordgrpc.NewServer(service, logger))
	healthServer := health.NewServer()
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
	grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)
	reflection.Register(grpcServer)

	lis, err := net.Listen("tcp", ":"+grpcPort)
	if err != nil {
		logger.Error("failed to listen", "port", grpcPort, "error", err)
		os.Exit(1)
	}

	go func() {
		logger.Info("ordering grpc server listening", "port", grpcPort)
		if err := grpcServer.Serve(lis); err != nil {
			logger.Error("grpc server stopped", "error", err)
		}
	}()

	go func() {
		logger.Info("ordering http server listening", "port", httpPort)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("http server stopped", "error", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	logger.Info("shutting down ordering server")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	_ = httpServer.Shutdown(shutdownCtx)
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

func getDurationEnv(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}
