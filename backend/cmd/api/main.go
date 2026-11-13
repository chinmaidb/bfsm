package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"schemeapp/backend/internal/application"
	"schemeapp/backend/internal/config"
	"schemeapp/backend/internal/httpapi"
	"schemeapp/backend/internal/postgres"
	"schemeapp/backend/internal/security"

	"github.com/joho/godotenv"
	"github.com/labstack/echo/v5"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		logger.Error("load .env file", "error", err)
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		logger.Error("configuration error", "error", err)
		os.Exit(1)
	}

	startupCtx, startupCancel := context.WithTimeout(context.Background(), 10*time.Second)
	db, err := postgres.Open(startupCtx, cfg.DatabaseURL)
	startupCancel()
	if err != nil {
		logger.Error("database startup failed", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	service := &application.Service{DB: db}
	if cfg.SuperAdminUsername != "" {
		provisionCtx, provisionCancel := context.WithTimeout(context.Background(), 15*time.Second)
		err := service.EnsureSuperAdmin(provisionCtx, cfg.SuperAdminUsername, cfg.SuperAdminPassword)
		provisionCancel()
		if err != nil {
			logger.Error("configured super admin setup failed", "error", err)
			os.Exit(1)
		}
		logger.Info("configured super admin is ready", "username", cfg.SuperAdminUsername)
	}

	minutes, err := strconv.Atoi(cfg.JWTLifetimeMinutes)
	if err != nil || minutes < 1 || minutes > 1440 {
		logger.Error("JWT_LIFETIME_MINUTES must be between 1 and 1440")
		os.Exit(1)
	}
	issuer, err := security.LoadTokenIssuer(cfg.JWTPrivateKey, cfg.JWTIssuer, time.Duration(minutes)*time.Minute)
	if err != nil {
		logger.Error("token signing configuration failed", "error", err)
		os.Exit(1)
	}

	server := httpapi.New(cfg, logger, service, issuer)
	if cfg.WebDir != "" {
		if err := httpapi.ServeFrontend(server, cfg.WebDir); err != nil {
			logger.Error("frontend setup failed", "error", err)
			os.Exit(1)
		}
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	logger.Info("starting API", "address", cfg.Address, "environment", cfg.Environment, "web_dir", cfg.WebDir)
	start := echo.StartConfig{
		Address:         cfg.Address,
		HideBanner:      true,
		GracefulTimeout: 10 * time.Second,
		BeforeServeFunc: func(s *http.Server) error {
			s.ReadHeaderTimeout = 5 * time.Second
			s.IdleTimeout = 60 * time.Second
			return nil
		},
	}
	if err := start.Start(ctx, server); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("server stopped unexpectedly", "error", err)
		os.Exit(1)
	}
}
