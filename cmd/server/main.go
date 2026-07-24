package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/helix-seller/helix-seller/internal/config"
	"github.com/helix-seller/helix-seller/internal/database"
	"github.com/helix-seller/helix-seller/internal/eventbus"
	"github.com/helix-seller/helix-seller/internal/handler"
	"github.com/helix-seller/helix-seller/internal/middleware"
	"github.com/helix-seller/helix-seller/internal/repository"
	"github.com/helix-seller/helix-seller/internal/service"
	"github.com/helix-seller/helix-seller/internal/websocket"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}

	logger := initLogger()
	defer logger.Sync()

	cfg := config.Load()

	// Database
	postgres, err := database.NewPostgres(cfg.DatabaseURL)
	if err != nil {
		logger.Fatal("Failed to connect to PostgreSQL", zap.Error(err))
	}
	defer postgres.Close()

	redisClient, err := database.NewRedis(cfg.RedisURL)
	if err != nil {
		logger.Fatal("Failed to connect to Redis", zap.Error(err))
	}
	defer redisClient.Close()

	// Event bus
	eb, err := eventbus.NewNatsEventBus(cfg.NATSURL, logger)
	if err != nil {
		logger.Warn("NATS not available, event bus disabled", zap.Error(err))
	}

	// Repositories
	userRepo := repository.NewUserRepo(postgres.Pool)
	merchantRepo := repository.NewMerchantRepo(postgres.Pool)
	customerRepo := repository.NewCustomerRepo(postgres.Pool)
	txRepo := repository.NewTransactionRepo(postgres.Pool)
	pmRepo := repository.NewPaymentMethodRepo(postgres.Pool)
	subscriptionRepo := repository.NewSubscriptionRepo(postgres.Pool)
	invoiceRepo := repository.NewInvoiceRepo(postgres.Pool)
	payoutRepo := repository.NewPayoutRepo(postgres.Pool)
	disputeRepo := repository.NewDisputeRepo(postgres.Pool)
	webhookConfigRepo := repository.NewWebhookConfigRepo(postgres.Pool)
	providerRepo := repository.NewProviderConfigRepo(postgres.Pool)
	auditRepo := repository.NewAuditLogRepo(postgres.Pool)

	// Services
	backgroundSvc := service.NewBackgroundService(postgres.Pool, logger, cfg.BackgroundWorkers, cfg.PollInterval)
	authSvc := service.NewAuthService(userRepo)
	jwtSvc, err := service.NewJWTService(cfg)
	if err != nil {
		logger.Fatal("Failed to initialize JWT service", zap.Error(err))
	}
	mfaSvc := service.NewMFAService()
	apiKeySvc := service.NewApiKeyService(postgres.Pool)
	paymentSvc := service.NewPaymentService(txRepo, pmRepo, eb, logger)
	subscriptionSvc := service.NewSubscriptionService(subscriptionRepo, eb, logger)
	invoiceSvc := service.NewInvoiceService(invoiceRepo, eb, logger)
	payoutSvc := service.NewPayoutService(payoutRepo, eb, logger)
	disputeSvc := service.NewDisputeService(disputeRepo, eb, logger)
	webhookSvc := service.NewWebhookService(webhookConfigRepo, logger)
	exchangeRateSvc := service.NewExchangeRateService(postgres.Pool, logger)
	analyticsSvc := service.NewAnalyticsService(postgres.Pool)
	billingSvc := service.NewBillingService(postgres.Pool, logger)

	// Handlers
	authHandler := handler.NewAuthHandler(authSvc, jwtSvc, mfaSvc, userRepo)
	userHandler := handler.NewUserHandler(userRepo)
	apiKeyHandler := handler.NewApiKeyHandler(apiKeySvc)
	merchantHandler := handler.NewMerchantHandler(merchantRepo)
	paymentHandler := handler.NewPaymentHandler(paymentSvc)
	customerHandler := handler.NewCustomerHandler(customerRepo, pmRepo)
	subscriptionHandler := handler.NewSubscriptionHandler(subscriptionSvc)
	invoiceHandler := handler.NewInvoiceHandler(invoiceSvc)
	payoutHandler := handler.NewPayoutHandler(payoutSvc)
	disputeHandler := handler.NewDisputeHandler(disputeSvc)
	webhookHandler := handler.NewWebhookHandler(webhookSvc)
	analyticsHandler := handler.NewAnalyticsHandler(analyticsSvc)
	providerHandler := handler.NewProviderHandler(providerRepo)
	paymentMethodHandler := handler.NewPaymentMethodHandler(pmRepo)
	exchangeRateHandler := handler.NewExchangeRateHandler(exchangeRateSvc)
	auditHandler := handler.NewAuditHandler(auditRepo)
	webhookIngressHandler := handler.NewWebhookIngressHandler(webhookSvc, eb, logger)
	billingHandler := handler.NewBillingHandler(billingSvc)
	healthHandler := handler.NewHealthHandler(postgres.Pool, redisClient.Client, logger)

	// WebSocket
	wsHub := websocket.NewHub(logger)
	go wsHub.Run()
	wsHandler := websocket.NewWSHandler(wsHub, logger)

	// Router
	router := handler.NewRouter(
		logger,
		authHandler,
		userHandler,
		apiKeyHandler,
		merchantHandler,
		paymentHandler,
		customerHandler,
		subscriptionHandler,
		invoiceHandler,
		payoutHandler,
		disputeHandler,
		webhookHandler,
		analyticsHandler,
		providerHandler,
		paymentMethodHandler,
		exchangeRateHandler,
		auditHandler,
		webhookIngressHandler,
		billingHandler,
		healthHandler,
		wsHandler,
	)

	// Apply middleware
	router.Use(middleware.Recovery(logger))
	router.Use(middleware.CORS())
	router.Use(middleware.RequestID())

	// Start background worker
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	backgroundSvc.Start(ctx)

	server := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.ServerHost, cfg.ServerPort),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		logger.Info("Starting server", zap.String("addr", server.Addr))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Failed to start server", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Fatal("Server forced to shutdown", zap.Error(err))
	}

	cancel()
	logger.Info("Server exited gracefully")
}

func initLogger() *zap.Logger {
	cfg := zap.Config{
		Level:       zap.NewAtomicLevelAt(zapcore.InfoLevel),
		Development: false,
		Encoding:    "json",
		EncoderConfig: zapcore.EncoderConfig{
			TimeKey:        "time",
			LevelKey:       "level",
			NameKey:        "logger",
			CallerKey:      "caller",
			MessageKey:     "msg",
			StacktraceKey:  "stacktrace",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.LowercaseLevelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeDuration: zapcore.SecondsDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		},
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}

	logger, err := cfg.Build()
	if err != nil {
		log.Fatal("Failed to initialize logger", err)
	}

	return logger
}
