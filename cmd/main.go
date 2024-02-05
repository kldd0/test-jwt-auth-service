package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"test-jwt-auth-service/internal/config"
	mwLogger "test-jwt-auth-service/internal/http-server/middleware/logger"
	"test-jwt-auth-service/internal/logger"
	"time"

	"log/slog"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
)

func main() {
	config := config.MustLoad()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log := logger.InitLogger(config.Env)

	log.Info(
		"starting jwt-auth-service",
		slog.String("env", config.Env),
		slog.String("ver", "1.0"),
	)
	log.Debug("debug messages are enabled")

	router := chi.NewRouter()

	router.Use(middleware.RequestID)
	router.Use(middleware.Logger)
	router.Use(mwLogger.New(log))
	router.Use(middleware.Recoverer)
	router.Use(middleware.URLFormat)

	// healthcheck route
	router.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=UTF-8")
		w.WriteHeader(http.StatusOK)

		_ = json.NewEncoder(w).Encode(map[string]bool{
			"pong": true,
		})
	})

	log.Info("starting http server", slog.String("address", config.Address))

	// server configuration
	srv := &http.Server{
		Addr:         config.HTTPServer.Address,
		Handler:      router,
		ReadTimeout:  config.HTTPServer.Timeout,
		WriteTimeout: config.HTTPServer.Timeout,
		IdleTimeout:  config.HTTPServer.IdleTimeout,
	}

	// listen to OS signals and gracefully shutdown HTTP server
	done := make(chan struct{})
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
		<-sigint

		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		log.Info("stopping server")

		if err := srv.Shutdown(ctx); err != nil {
			log.Info("http server shutdown error", logger.Err(err))
		}

		close(done)
	}()

	// start http server
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Error("http server ListenAndServe error:", logger.Err(err))
	}

	<-done

	log.Info("http server stopped")
}
